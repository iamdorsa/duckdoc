package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/russross/blackfriday/v2"
)

type TreeNode struct {
	Name         string
	Path         string
	Children     []*TreeNode
	CompletePath string
}

type TOCItem struct {
	Level int
	Text  string
	ID    string
}

type PageData struct {
	Title       string
	Navbar      *TreeNode
	Content     template.HTML
	TOC         []TOCItem
	CurrentFile string
}

// templateFuncs is populated at request time to carry currentFile for active-state highlighting
func makeTemplateFuncs(currentFile string) template.FuncMap {
	return template.FuncMap{
		"defineTree": func(node *TreeNode) template.HTML {
			return template.HTML(renderTreeHTML(node, currentFile))
		},
	}
}

func main() {
	http.HandleFunc("/", serveMarkdown)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/content/", http.StripPrefix("/content/", http.FileServer(http.Dir("content"))))

	port := ":8080"
	log.Printf("Server started on http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func serveMarkdown(w http.ResponseWriter, r *http.Request) {
	navbarTree, err := buildNavTree("content")
	if err != nil {
		log.Printf("Error building nav tree: %v", err)
		http.Error(w, "Error reading content", http.StatusInternalServerError)
		return
	}

	if q := r.URL.Query().Get("search"); q != "" {
		filterTree(navbarTree, q)
	}

	requestedPath := r.URL.Path
	var content, relPath string
	var isSwagger bool

	if requestedPath == "/" {
		relPath = "README.md"
		if b, err := os.ReadFile("README.md"); err == nil {
			content = string(b)
		} else {
			content = "# Welcome to DuckDoc\n\nThe README file could not be found."
		}
	} else {
		relPath = strings.TrimPrefix(requestedPath, "/")
		if b, err := os.ReadFile(filepath.Join("content", relPath)); err == nil {
			content = string(b)
			ext := filepath.Ext(relPath)
			if ext == ".yaml" || ext == ".yml" {
				trimmed := strings.TrimSpace(content)
				if strings.HasPrefix(trimmed, "openapi:") || strings.HasPrefix(trimmed, "swagger:") {
					isSwagger = true
				}
			}
		} else {
			content = "# File not found\n\nThe requested file could not be found."
		}
	}

	funcs := makeTemplateFuncs(relPath)

	if isSwagger {
		renderSwagger(w, relPath, navbarTree, funcs)
		return
	}

	toc := extractTOC(content)
	processed := processImagePaths(content, relPath)
	htmlContent := template.HTML(blackfriday.Run([]byte(processed), blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs)))

	tmpl, err := template.New("layout").Funcs(funcs).ParseFiles("templates/layout.html", "templates/sidebar.html")
	if err != nil {
		log.Printf("Template parsing error: %v", err)
		http.Error(w, "Error loading templates", http.StatusInternalServerError)
		return
	}

	page := PageData{
		Title:       "DuckDoc",
		Navbar:      navbarTree,
		Content:     htmlContent,
		TOC:         toc,
		CurrentFile: relPath,
	}

	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

func buildNavTree(dir string) (*TreeNode, error) {
	root := &TreeNode{Name: "root", Path: "", Children: []*TreeNode{}}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		parts := strings.Split(filepath.ToSlash(relPath), "/")
		current := root
		for i, part := range parts {
			found := false
			for _, child := range current.Children {
				if child.Name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				var nodePath string
				if i == len(parts)-1 {
					nodePath = "/" + relPath
				} else {
					nodePath = "/" + strings.Join(parts[:i+1], "/")
				}
				completePath := strings.Join(parts[:i+1], "/")
				newNode := &TreeNode{
					Name:         part,
					Path:         nodePath,
					Children:     []*TreeNode{},
					CompletePath: completePath,
				}
				current.Children = append(current.Children, newNode)
				current = newNode
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	collapseChains(root)
	return root, nil
}

// collapseChains merges folder nodes that have exactly one child folder into one label.
// e.g. "atlas / docs" with only folder children becomes "atlas/docs"
func collapseChains(node *TreeNode) {
	for _, child := range node.Children {
		collapseChains(child)
	}

	for i, child := range node.Children {
		// Only collapse if the child is a pure folder (no file children) with exactly one child
		for len(child.Children) == 1 && len(child.Children[0].Children) > 0 {
			only := child.Children[0]
			child.Name = child.Name + " / " + only.Name
			child.Path = only.Path
			child.CompletePath = only.CompletePath
			child.Children = only.Children
		}
		node.Children[i] = child
	}
}

// displayName strips the file extension for cleaner sidebar labels
func displayName(name string) string {
	ext := filepath.Ext(name)
	if ext == ".md" || ext == ".yaml" || ext == ".yml" {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

func renderTreeHTML(node *TreeNode, currentFile string) string {
	if node == nil {
		return ""
	}
	if node.Name == "root" {
		var sb strings.Builder
		for _, child := range node.Children {
			sb.WriteString(renderTreeHTML(child, currentFile))
		}
		return sb.String()
	}

	fullPath := template.HTMLEscapeString(node.CompletePath)
	name := template.HTMLEscapeString(displayName(node.Name))

	if len(node.Children) > 0 {
		// Check if any descendant is the active file to pre-expand the folder
		active := containsFile(node, currentFile)
		activeClass := ""
		if active {
			activeClass = " active-ancestor"
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `<li class="folder%s" data-full-path="%s"><span class="folder-name">%s</span><ul>`, activeClass, fullPath, name)
		for _, child := range node.Children {
			sb.WriteString(renderTreeHTML(child, currentFile))
		}
		sb.WriteString("</ul></li>")
		return sb.String()
	}

	path := template.HTMLEscapeString(node.Path)
	isActive := strings.TrimPrefix(node.Path, "/") == currentFile
	activeClass := ""
	if isActive {
		activeClass = ` class="active"`
	}
	return fmt.Sprintf(`<li class="file" data-full-path="%s"><a href="%s"%s>%s</a></li>`, fullPath, path, activeClass, name)
}

// containsFile reports whether any file descendant matches currentFile
func containsFile(node *TreeNode, currentFile string) bool {
	if len(node.Children) == 0 {
		return strings.TrimPrefix(node.Path, "/") == currentFile
	}
	for _, child := range node.Children {
		if containsFile(child, currentFile) {
			return true
		}
	}
	return false
}

// stripInlineMarkdown removes inline backtick spans, keeping the inner text,
// so that "`foo`" becomes "foo" — matching blackfriday's AutoHeadingIDs behaviour.
var backtickRegex = regexp.MustCompile("`[^`]*`")

// nonAlphanumHyphen matches anything that is not a lowercase letter, digit, or hyphen.
var nonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]+`)

func headingToID(text string) string {
	// 1. Strip inline code backticks but keep inner text.
	id := backtickRegex.ReplaceAllStringFunc(text, func(s string) string {
		return s[1 : len(s)-1]
	})
	// 2. Lowercase.
	id = strings.ToLower(id)
	// 3. Replace spaces and word-separating punctuation with hyphens so that
	//    "ci.yml" → "ci-yml" (matching blackfriday's slugifier).
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, ".", "-")
	// 4. Remove every remaining character that is not [a-z0-9-].
	id = nonAlphanumHyphen.ReplaceAllString(id, "")
	// 5. Collapse consecutive hyphens produced by the steps above.
	id = regexp.MustCompile(`-{2,}`).ReplaceAllString(id, "-")
	// 6. Trim leading/trailing hyphens.
	id = strings.Trim(id, "-")
	return id
}

func extractTOC(content string) []TOCItem {
	var toc []TOCItem
	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.*)`)
	for _, line := range strings.Split(content, "\n") {
		if m := headerRegex.FindStringSubmatch(line); m != nil {
			text := m[2]
			toc = append(toc, TOCItem{Level: len(m[1]), Text: text, ID: headingToID(text)})
		}
	}
	return toc
}

func filterTree(node *TreeNode, query string) bool {
	nodeMatches := matchesQuery(node.Name, node.CompletePath, query)

	var matched []*TreeNode
	for _, child := range node.Children {
		if filterTree(child, query) {
			matched = append(matched, child)
		}
	}
	node.Children = matched

	return nodeMatches || len(matched) > 0
}

func matchesQuery(name, completePath, query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(name), q) ||
		strings.Contains(strings.ToLower(completePath), q)
}

func processImagePaths(content, relPath string) string {
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	var baseDir string
	if relPath == "README.md" {
		baseDir = "content"
	} else {
		baseDir = filepath.Dir(relPath)
		if baseDir == "." {
			baseDir = ""
		}
	}

	return imageRegex.ReplaceAllStringFunc(content, func(match string) string {
		m := imageRegex.FindStringSubmatch(match)
		if len(m) != 3 {
			return match
		}
		altText, imagePath := m[1], m[2]

		if strings.HasPrefix(imagePath, "http://") ||
			strings.HasPrefix(imagePath, "https://") ||
			strings.HasPrefix(imagePath, "/") {
			return match
		}

		imagePath = strings.TrimPrefix(imagePath, "./")

		var newPath string
		if baseDir == "" || relPath == "README.md" {
			newPath = "/content/" + imagePath
		} else {
			newPath = "/content/" + baseDir + "/" + imagePath
		}

		return "![" + altText + "](" + newPath + ")"
	})
}

func renderSwagger(w http.ResponseWriter, relPath string, navbarTree *TreeNode, funcs template.FuncMap) {
	tmpl, err := template.New("layout").Funcs(funcs).ParseFiles("templates/layout.html", "templates/sidebar.html", "templates/swagger.html")
	if err != nil {
		log.Printf("Template parsing error: %v", err)
		http.Error(w, "Error loading templates", http.StatusInternalServerError)
		return
	}

	page := PageData{
		Title:       "API Documentation",
		Navbar:      navbarTree,
		Content:     "",
		TOC:         []TOCItem{},
		CurrentFile: relPath,
	}

	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}
