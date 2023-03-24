SHELL=/bin/bash

# https://www.gnu.org/software/make/manual/html_node/Setting.html
# https://opensource.com/article/18/8/what-how-makefile

CGO_ENABLED=0
GOCMD=go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GORUN := $(GOCMD) run
CURRENT_DIR := $(shell pwd)

SYNC_FILE="sync.json"

# VERSION := $(shell cat version.txt)
# GIT_VERSION := $(shell git describe --tags --always)
# BUILD := $(shell git rev-parse --short HEAD)
BUILD_DATE := $(shell date --rfc-3339=date)

all: sync

mods:
	hugo mod tidy
	hugo mod vendor
	hugo mod verify

lint:
	golangci-lint run --timeout=5m --skip-dirs vendor ./...

format: $(SOURCE_LIST)
	find . -name \*.go -not -path vendor -not -path target -exec goimports -w {} \;

sync:
	echo "this will export obsidian markdown files to hugo"
	go run sync.go .

copy-theme:
	# cp -R themes/gruvhugo/exampleSite/* .

	# https://themes.gohugo.io/themes/beautifulhugo/
	# use example site
	# cp -r themes/beautifulhugo/exampleSite/* . -iv

	# site preview
	# hugo server -t hugo-theme-techdoc
	# deploy site
	# hugo -t hugo-theme-techdoc -d public_html
	# dev site
	# cd /path/to/dir/themes/hugo-theme-techdoc/exampleSite
	# hugo server --themesDir ../..

	# install
	# https://digital-garden-hugo-theme.vercel.app/articles/installation/
	# TODO: need to use npm to complete setup! (do it in the theme folder then)
	# hugo server --themesDir ../..

	echo "null"

install-themes:
# 	git submodule add https://github.com/wowchemy/hugo-second-brain-theme themes/second-brain
# 	git submodule add https://github.com/wowchemy/hugo-documentation-theme themes/documentation
	git submodule add https://github.com/alex-shpak/hugo-book themes/hugo-book
# 	git submodule add https://github.com/rhazdon/hugo-theme-hello-friend-ng themes/hello-friend-ng
# 	git submodule add https://github.com/h-enk/doks themes/doks
# 	git submodule add https://github.com/halogenica/beautifulhugo themes/beautifulhugo
# 	git submodule add https://gitlab.com/avron/gruvhugo.git themes/gruvhugo
# 	git submodule add https://github.com/thingsym/hugo-theme-techdoc.git themes/hugo-theme-techdoc
# 	git submodule add https://github.com/spf13/hyde themes/hyde
# 	git submodule add https://github.com/nanxiaobei/hugo-paper themes/paper
# 	git submodule add https://github.com/adityatelange/hugo-PaperMod themes/papermod
# 	git submodule add https://github.com/victoriadrake/hugo-theme-introduction themes/introduction
# 	git submodule add https://github.com/google/docsy themes/docsy
# 	git submodule add https://github.com/apvarun/digital-garden-hugo-theme.git themes/digitalgarden
# 	git submodule add https://gitlab.com/rmaguiar/hugo-theme-color-your-world themes/hugo-theme-color-your-world
# 	git submodule add https://github.com/spech66/bootstrap-bp-hugo-startpage themes/bootstrap-bp-hugo-startpage
# 	git submodule add https://github.com/McShelby/hugo-theme-relearn themes/hugo-theme-relearn

nuke:
# 	rm -rf archetypes/*
# 	rm -rf assets/*
# 	rm -rf config/*
	rm -rf content/*
# 	rm -rf layouts/*
# 	rm -rf public/*
# 	rm -rf static/*
# 	rm -rf data/*
# 	rm -rf resources/*

extract-example-%: THEME_DIR=$*
extract-example-%:
	cp -R themes/hugo-book/exampleSite/content.en/* ./content

dev: hugo serve

hugo:
	hugo --gc --verbose --cleanDestinationDir --forceSyncStatic --ignoreCache --noBuildLock

serve:
	cd public ; /usr/local/bin/python3 -m http.server

server:
	hugo server --gc --verbose --cleanDestinationDir --forceSyncStatic --noHTTPCache --panicOnWarning

serve-%: THEME_DIR=$*
serve-%:
	cd themes/$(THEME_DIR)/exampleSite ; hugo server --verbose --themesDir ../..
