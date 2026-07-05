package routes

import (
	"net/http"

	"github.com/jxc2000-b/git-preview/config"
)

func Handlers(c *config.Config) *http.ServeMux {
	mux := http.NewServeMux()
	d := deps{c}

	mux.HandleFunc("GET /", d.Index)
	mux.HandleFunc("GET /static/{file}", d.ServeStatic)
	mux.HandleFunc("GET /{name}", d.RepoIndex)
	mux.HandleFunc("GET /{name}/tree/{ref}/{rest...}", d.RepoTree)
	mux.HandleFunc("GET /{name}/blob/{ref}/{rest...}", d.FileContent)
	mux.HandleFunc("GET /{name}/log/{ref}", d.Log)
	mux.HandleFunc("GET /{name}/commit/{ref}", d.Diff)
	mux.HandleFunc("GET /{name}/showcase/{ref}/{rest...}", d.ShowcaseAsset)
	mux.HandleFunc("GET /{name}/{rest...}", d.RepoIndex)

	return mux
}
