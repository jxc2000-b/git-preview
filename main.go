package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/jxc2000-b/git-preview/config"
	"github.com/jxc2000-b/git-preview/export"
	"github.com/jxc2000-b/git-preview/routes"
)

func main() {
	var cfg, exportDir string
	flag.StringVar(&cfg, "config", "./config.yaml", "path to config file")
	flag.StringVar(&exportDir, "export", "", "export a static site to this directory instead of serving")
	flag.Parse()

	c, err := config.Read(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Fall back to the assets baked into the binary when the config
	// doesn't point at template/static dirs on disk.
	if c.Dirs.Templates == "" || c.Dirs.Static == "" {
		tpl, static, err := materializeAssets()
		if err != nil {
			log.Fatalf("embedded assets: %s", err)
		}
		if c.Dirs.Templates == "" {
			c.Dirs.Templates = tpl
		}
		if c.Dirs.Static == "" {
			c.Dirs.Static = static
		}
	}

	if exportDir != "" {
		if err := export.Run(c, exportDir); err != nil {
			log.Fatal(err)
		}
		log.Println("exported static site to", exportDir)
		return
	}

	if err := UnveilPaths([]string{
		c.Dirs.Static,
		c.Repo.ScanPath,
		c.Dirs.Templates,
	},
		"r"); err != nil {
		log.Fatalf("unveil: %s", err)
	}

	mux := routes.Handlers(c)
	addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
	log.Println("starting server on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
