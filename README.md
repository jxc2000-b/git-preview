# Originally Forked from [legit](https://github.com/icyphox/legit)
An opinionated take on what a static git web frontend could be. Preview you repository in a customizeable fashion for easy deployment as a stripped-down silly showcase website. 

example: https://jxc2000-b.github.io/git-preview/

host [images](https://jxc2000-b.github.io/git-preview/blob/main/main.go/) [alongside](https://jxc2000-b.github.io/git-preview/blob/main/config.yaml/) your code, [interactive html demos](https://jxc2000-b.github.io/git-preview/blob/main/go.mod/) and truncate your code securely.
## Adding it to a new project

```sh
cd my-app
mkdir preview-repos # parent folder that holds the apps you want to preview
git clone --bare . preview-repos/my-app # clone your app here and re-push here to publish updates
```

## Create a conifg.yaml file, example config: 

`config.yaml`

```yaml
repo:
  scanPath: ./preview-repos
  readme:
    - readme
    - README.md
  mainBranch:
    - master
    - main

meta: 
  title: my-app preview # site-wide metadata (title shown on browser tab)
  description: my-app description # search engines, link previews...
  syntaxHighlight: monokailight # OR github-dark, dracula, nord, onedark, rose-pine-moon, tokyonight-night...

server:
  name: localhost
  host: 127.0.0.1
  port: 5555
```

## Install git preview to your app

```sh
cd my-app
go install github.com/jxc2000-b/git-preview@latest
```
## Create a .showcode folder and create a showcase.yaml file, example below 

```sh
cd my-app/.showcase
```

`showcase.yaml`

```yaml
# Default preview setting
defaultPreview: full # can be full, truncated or none

# Displayed on the repository landing page
landing:
  - asset: images/go_users.jpeg
    caption: You can add also images like this

# Displayed when viewing specific files
files:
  "config.yaml":
    preview: full #individual overrides default
    assets:
      - asset: images/beluga.jpeg
        caption: Imagine these were important files detailing features or architecture 
  "main.go":
    preview: full
    assets:
      - asset: images/huh.jpeg
        caption: Imagine this were something cool about this file
  "showcase/showcase.go":
    preview: full
    assets: 
      - asset: html/interactive.html
        caption: You can add interactive demos too
        height: 64

# Displayed on a particular commit page
commits:
  "bdb3273":
    - asset: images/andale.jpeg
      caption: Imagine this were the result introduced by this commit
```
## Commit and push your changes to your bare clone

```sh
git add .showcase
git commit -m "Add project showcase"
git push preview-repos/my-app main
```

## Generate your preview files
```sh
git-preview -config config.yaml
# browse at :5555
git-preview -config config.yaml -export showcase/out # static build
```

## Local preview of an export
```sh
cd showcase/out && python3 -m http.server 5556
```
> [!IMPORTANT]
> There are multiple issues that will arise when setting up github pages this way, I recommend you copy the workaround [workflow file](.github/workflows/static.yml) used in this repository verbatim for now while I ship fixes.

## Example deploy with Github Pages
```yaml
- uses: actions/setup-go@v5
- run: go install github.com/jxc2000-b/git-preview@latest
- run: git-preview -config preview.yaml -export showcase/out
- run: npx wrangler pages deploy showcase/out --project-name=<name>
```

Much thanks to the original developers @icyphox
