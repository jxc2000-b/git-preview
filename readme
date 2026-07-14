# Forked from [legit](https://github.com/icyphox/legit)

------
An opinionated take on what a git web frontend could be. Preview you repository in a customizeable fashion for easy deployment as a stripped-down showcase static website. 


## Adding it to a new project

```sh
cd my-app
mkdir preview-repos
git clone --bare . preview-repos/my-app # re-push here to publish updates

```

## Create a conifg.yaml file, example config: 

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
  title: my-app preview
  description: git web with showcase
  syntaxHighlight: monokailight

server:
  name: localhost
  host: 127.0.0.1
  port: 5555
```

## Install git preview

```sh
go install github.com/jxc2000-b/git-preview@latest
```

## Generate your preview files
```sh
git-preview -config preview.yaml
#browse at :5555
git-preview -config preview.yaml -export .showcase/out # static build
```

## Local preview of an export
```sh
cd .showcase/out && python3 -m http.server 5556
```

## Example deploy with Github Pages

```yaml
- uses: actions/setup-go@v5
- run: go install github.com/jxc2000-b/git-preview@latest
- run: git-preview -config preview.yaml -export .showcase/out
- run: npx wrangler pages deploy .showcase/out --project-name=<name>
```
