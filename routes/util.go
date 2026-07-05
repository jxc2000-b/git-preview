package routes

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxc2000-b/git-preview/git"
)

func isGoModule(gr *git.GitRepo) bool {
	_, err := gr.FileContent("go.mod")
	return err == nil
}

func getDisplayName(name string) string {
	return strings.TrimSuffix(name, ".git")
}

func getDescription(path string) (desc string) {
	db, err := os.ReadFile(filepath.Join(path, "description"))
	if err == nil {
		desc = string(db)
	} else {
		desc = ""
	}
	return
}

func (d *deps) isUnlisted(name string) bool {
	for _, i := range d.c.Repo.Unlisted {
		if name == i {
			return true
		}
	}

	return false
}

func (d *deps) isIgnored(name string) bool {
	for _, i := range d.c.Repo.Ignore {
		if name == i {
			return true
		}
	}

	return false
}

// soloRepo returns the repo name if the scan path holds exactly one
// visible repo — the site then behaves as a single-project preview
// (no index page, no "all repos" backlink).
func (d *deps) soloRepo() string {
	dirs, err := os.ReadDir(d.c.Repo.ScanPath)
	if err != nil {
		return ""
	}
	solo := ""
	for _, dir := range dirs {
		if !dir.IsDir() || d.isIgnored(dir.Name()) {
			continue
		}
		if solo != "" {
			return ""
		}
		solo = dir.Name()
	}
	return solo
}

type repoInfo struct {
	Git      *git.GitRepo
	Path     string
	Category string
}

func (d *deps) getAllRepos() ([]repoInfo, error) {
	repos := []repoInfo{}
	max := strings.Count(d.c.Repo.ScanPath, string(os.PathSeparator)) + 2

	err := filepath.WalkDir(d.c.Repo.ScanPath, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if de.IsDir() {
			// Check if we've exceeded our recursion depth
			if strings.Count(path, string(os.PathSeparator)) > max {
				return fs.SkipDir
			}

			if d.isIgnored(path) {
				return fs.SkipDir
			}

			// A bare repo should always have at least a HEAD file, if it
			// doesn't we can continue recursing
			if _, err := os.Lstat(filepath.Join(path, "HEAD")); err == nil {
				repo, err := git.Open(path, "")
				if err != nil {
					log.Println(err)
				} else {
					relpath, _ := filepath.Rel(d.c.Repo.ScanPath, path)
					repos = append(repos, repoInfo{
						Git:      repo,
						Path:     relpath,
						Category: d.category(path),
					})
					// Since we found a Git repo, we don't want to recurse
					// further
					return fs.SkipDir
				}
			}
		}
		return nil
	})

	return repos, err
}

func (d *deps) category(path string) string {
	return strings.TrimPrefix(filepath.Dir(strings.TrimPrefix(path, d.c.Repo.ScanPath)), string(os.PathSeparator))
}

