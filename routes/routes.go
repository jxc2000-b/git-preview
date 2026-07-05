package routes

import (
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jxc2000-b/git-preview/config"
	"github.com/jxc2000-b/git-preview/git"
	"github.com/jxc2000-b/git-preview/showcase"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dustin/go-humanize"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type deps struct {
	c *config.Config
}

func (d *deps) Index(w http.ResponseWriter, r *http.Request) {
	// Single-project mode: the site root *is* the repo.
	if solo := d.soloRepo(); solo != "" {
		r.SetPathValue("name", solo)
		d.RepoIndex(w, r)
		return
	}

	dirs, err := os.ReadDir(d.c.Repo.ScanPath)
	if err != nil {
		d.Write500(w)
		log.Printf("reading scan path: %s", err)
		return
	}

	type info struct {
		DisplayName, Name, Desc, Idle string
		d                             time.Time
	}

	infos := []info{}

	for _, dir := range dirs {
		name := dir.Name()
		if !dir.IsDir() || d.isIgnored(name) || d.isUnlisted(name) {
			continue
		}

		path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
		if err != nil {
			log.Printf("securejoin error: %v", err)
			d.Write404(w)
			return
		}

		gr, err := git.Open(path, "")
		if err != nil {
			log.Println(err)
			continue
		}

		c, err := gr.LastCommit()
		if err != nil {
			d.Write500(w)
			log.Println(err)
			return
		}

		infos = append(infos, info{
			DisplayName: getDisplayName(name),
			Name:        name,
			Desc:        getDescription(path),
			Idle:        humanize.Time(c.Author.When),
			d:           c.Author.When,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[j].d.Before(infos[i].d)
	})

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})
	data["meta"] = d.c.Meta
	data["info"] = infos

	if err := t.ExecuteTemplate(w, "index", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) RepoIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	name = filepath.Clean(name)
	path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}

	gr, err := git.Open(path, "")
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	var readmeContent template.HTML
	for _, readme := range d.c.Repo.Readme {
		ext := filepath.Ext(readme)
		content, _ := gr.FileContent(readme)
		if len(content) > 0 {
			switch ext {
			case ".md", ".mkd", ".markdown":
				unsafe := blackfriday.Run(
					[]byte(content),
					blackfriday.WithExtensions(blackfriday.CommonExtensions),
				)
				html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
				readmeContent = template.HTML(html)
			default:
				safe := bluemonday.UGCPolicy().SanitizeBytes([]byte(content))
				readmeContent = template.HTML(
					fmt.Sprintf(`<pre>%s</pre>`, safe),
				)
			}
			break
		}
	}

	if readmeContent == "" {
		log.Printf("no readme found for %s", name)
	}

	mainBranch, err := gr.FindMainBranch(d.c.Repo.MainBranch)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	if len(commits) >= 3 {
		commits = commits[:3]
	}

	data := make(map[string]any)
	data["name"] = name
	data["displayname"] = getDisplayName(name)
	data["ref"] = mainBranch
	data["readme"] = readmeContent
	data["commits"] = commits
	data["desc"] = getDescription(path)
	data["servername"] = d.c.Server.Name
	data["meta"] = d.c.Meta
	data["gomod"] = isGoModule(gr)
	data["single"] = d.soloRepo() != ""

	if sc, err := showcase.Load(gr); err != nil {
		log.Printf("showcase manifest for %s: %s", name, err)
	} else if sc != nil && len(sc.Landing) > 0 {
		data["showcase"] = sc.Landing
		data["showcaseref"] = mainBranch
	}

	if err := t.ExecuteTemplate(w, "repo", data); err != nil {
		log.Println(err)
		return
	}

	return
}

func (d *deps) RepoTree(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := r.PathValue("rest")
	ref := r.PathValue("ref")

	name = filepath.Clean(name)
	path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	files, err := gr.FileTree(treePath)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	// The .showcase sidecar dir is plumbing for the showcase galleries;
	// keep it out of the root tree listing.
	if treePath == "" {
		filtered := files[:0]
		for _, f := range files {
			if f.Name != showcase.Dir {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	data := make(map[string]any)
	data["name"] = name
	data["displayname"] = getDisplayName(name)
	data["ref"] = ref
	data["parent"] = treePath
	data["desc"] = getDescription(path)
	data["dotdot"] = filepath.Dir(treePath)
	data["single"] = d.soloRepo() != ""

	d.listFiles(files, data, w)
	return
}

func (d *deps) FileContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := r.PathValue("rest")
	ref := r.PathValue("ref")

	name = filepath.Clean(name)
	path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}

	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	contents, err := gr.FileContent(treePath)
	if err != nil {
		d.Write500(w)
		return
	}
	data := make(map[string]any)
	data["name"] = name
	data["displayname"] = getDisplayName(name)
	data["ref"] = ref
	data["desc"] = getDescription(path)
	data["path"] = treePath
	data["single"] = d.soloRepo() != ""

	sc, err := showcase.Load(gr)
	if err != nil {
		log.Printf("showcase manifest for %s: %s", name, err)
	}
	if assets := sc.ForFile(treePath); len(assets) > 0 {
		data["showcase"] = assets
		data["showcaseref"] = ref
	}

	// Blob views are previews by default: 20 lines unless the manifest
	// grants "full" (or forbids with "none"). Enforced server-side, so
	// hidden lines are never sent.
	const maxPreviewLines = 20

	switch sc.PreviewMode(treePath) {
	case "none":
		data["hidden"] = true
		d.showFile("", data, w)
		return
	case "truncated":
		if t, cut := truncateLines(contents, maxPreviewLines); cut {
			contents = t
			data["truncated"] = true
		}
	}

	if d.c.Meta.SyntaxHighlight == "" {
		d.showFile(contents, data, w)
	} else {
		d.showFileWithHighlight(treePath, contents, data, w)
	}
}

// truncateLines clips s to at most n lines, reporting whether it cut.
func truncateLines(s string, n int) (string, bool) {
	count := 0
	for i := range s {
		if s[i] == '\n' {
			count++
			if count == n {
				return s[:i+1], i+1 < len(s)
			}
		}
	}
	return s, false
}

// ShowcaseAsset serves raw bytes of a file under the repo's .showcase/
// directory, for use by the showcase gallery (images, video, iframes).
func (d *deps) ShowcaseAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := r.PathValue("ref")
	file := path.Clean(r.PathValue("rest"))
	if file == "." || file == ".." || strings.HasPrefix(file, "../") || strings.HasPrefix(file, "/") {
		d.Write404(w)
		return
	}

	name = filepath.Clean(name)
	repoPath, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}

	gr, err := git.Open(repoPath, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	contents, err := gr.FileContentBytes(path.Join(showcase.Dir, file))
	if err != nil {
		d.Write404(w)
		return
	}

	ctype := mime.TypeByExtension(path.Ext(file))
	if ctype == "" {
		ctype = http.DetectContentType(contents)
	}

	// Interactive HTML runs sandboxed: scripts may run, but the frame
	// gets a unique origin and no access to the parent site.
	if strings.HasPrefix(ctype, "text/html") {
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts")
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(contents)
}

func (d *deps) Log(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := r.PathValue("ref")

	path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}

	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	type logEntry struct {
		Hash    string
		Message string
		Age     string
		Files   int
		Add     int
		Del     int
		Decor   []git.Decoration
	}

	decorations := gr.Decorations()

	entries := make([]logEntry, 0, len(commits))
	for _, c := range commits {
		e := logEntry{
			Hash:    c.Hash.String(),
			Message: strings.SplitN(strings.TrimSpace(c.Message), "\n", 2)[0],
			Age:     c.Author.When.Format("02 Jan 2006"),
			Decor:   decorations[c.Hash.String()],
		}
		if stats, err := c.Stats(); err == nil {
			e.Files = len(stats)
			for _, s := range stats {
				e.Add += s.Addition
				e.Del += s.Deletion
			}
		}
		entries = append(entries, e)
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})
	data["commits"] = entries
	data["meta"] = d.c.Meta
	data["name"] = name
	data["displayname"] = getDisplayName(name)
	data["ref"] = ref
	data["desc"] = getDescription(path)
	data["log"] = true
	data["single"] = d.soloRepo() != ""

	if err := t.ExecuteTemplate(w, "log", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) Diff(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := r.PathValue("ref")

	path, err := securejoin.SecureJoin(d.c.Repo.ScanPath, name)
	if err != nil {
		log.Printf("securejoin error: %v", err)
		d.Write404(w)
		return
	}
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	diff, err := gr.Diff()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	// The commit page is a *preview*: cap files and lines server-side so
	// undisclosed changes are never shipped in the response.
	const maxPreviewFiles = 3
	const maxPreviewLines = 10

	type previewFrag struct {
		Header string
		Lines  []gitdiff.Line
	}
	type previewDiff struct {
		Name      string
		IsNew     bool
		IsDelete  bool
		IsBinary  bool
		Fragments []previewFrag
		Truncated bool
	}

	shown := diff.Diff
	hiddenFiles := 0
	if len(shown) > maxPreviewFiles {
		hiddenFiles = len(shown) - maxPreviewFiles
		shown = shown[:maxPreviewFiles]
	}

	preview := make([]previewDiff, 0, len(shown))
	for _, fd := range shown {
		p := previewDiff{
			Name:     fd.Name.New,
			IsNew:    fd.IsNew,
			IsDelete: fd.IsDelete,
			IsBinary: fd.IsBinary,
		}
		if p.Name == "" {
			p.Name = fd.Name.Old
		}
		remaining := maxPreviewLines
		for _, tf := range fd.TextFragments {
			if remaining <= 0 {
				p.Truncated = true
				break
			}
			lines := tf.Lines
			if len(lines) > remaining {
				lines = lines[:remaining]
				p.Truncated = true
			}
			remaining -= len(lines)
			p.Fragments = append(p.Fragments, previewFrag{Header: tf.Header, Lines: lines})
		}
		preview = append(preview, p)
	}

	data := make(map[string]interface{})

	data["commit"] = diff.Commit
	data["stat"] = diff.Stat
	data["diff"] = preview
	data["hiddenfiles"] = hiddenFiles
	data["meta"] = d.c.Meta
	data["name"] = name
	// Don't echo the commit hash back as the displayed ref; the commit
	// page is a preview and shouldn't surface hashes anywhere.
	displayRef := ref
	if headRepo, err := git.Open(path, ""); err == nil {
		if mainBranch, err := headRepo.FindMainBranch(d.c.Repo.MainBranch); err == nil {
			displayRef = mainBranch
		}

		// Load the manifest from HEAD, not the viewed commit, so showcase
		// entries added later still annotate older commits.
		if sc, err := showcase.Load(headRepo); err != nil {
			log.Printf("showcase manifest for %s: %s", name, err)
		} else if assets := sc.ForCommit(diff.Commit.This); len(assets) > 0 {
			data["showcase"] = assets
			data["showcaseref"] = displayRef
		}
	}
	data["displayname"] = getDisplayName(name)
	data["ref"] = displayRef
	data["single"] = d.soloRepo() != ""
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "commit", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) ServeStatic(w http.ResponseWriter, r *http.Request) {
	f := r.PathValue("file")
	f = filepath.Clean(f)
	f, err := securejoin.SecureJoin(d.c.Dirs.Static, f)
	if err != nil {
		d.Write404(w)
		return
	}

	http.ServeFile(w, r, f)
}
