package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	// "log"
	"mime"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	gast "github.com/yuin/goldmark/ast"
	log "github.com/sirupsen/logrus"
	parser2 "github.com/gohugoio/hugo/parser"
)

type SyncPair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type SyncData struct {
	Label string `json:"label"`
	Pairs []SyncPair `json:"pairs"`
}

type SyncInfo struct {
	Data []SyncData `json:"data"`
	HomeDir string `json:"-"`
}

type FrontMatter struct {
	Draft bool `yaml:"draft"`
	Published bool `yaml:"published"`
	Date string `yaml:"date"`
	Tags []string `yaml:"tags"`
}

type CalloutAttrs struct {
	Type string `json:"type"`
	Collapsed bool `json:"collapsed"`
	Title string `json:"title,omitempty"`
}

const syncFileName = "sync.json"

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func main() {
	// rootCmd.Flags().StringP("sync-pairs", "p", "", "JSON transfer pair metadata")
	// rootCmd.MarkFlagRequired("sync-pairs")

	Execute()
}

var rootCmd = &cobra.Command{
	Use:   "obsidian2hugo",
	Short: "Finds obsidian pages marked as `published: true` and copies the files into the hugo directory",
	Long: `This tool was created to be able to export blog posts created inside obsidian for the usage inside a hugo blog.`,

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// var limit string
		var usr *user.User
		var syncPath string
		var syncMeta SyncInfo

		if usr, err = user.Current(); err != nil {
			log.Fatalf("unable to get current user")
		}

		// if syncPath, err = cmd.Flags().GetString("sync-pairs"); err != nil {
		// 	log.Fatal("`sync-pairs` flag missing")
		// }
		// if syncFile, err = os.Open(syncPath); err != nil {
		// 	log.Fatal("unable to open sync file for path data")
		// }
		// defer syncFile.Close()

		for _, dir := range cmd.Flags().Args() {
			if syncPath, err = filepath.Abs(dir); err != nil {
				log.Fatal(err)
			}
			if _, err = os.Stat(syncPath); err != nil {
				log.Fatal(err)
			}
			log.Infof("path=%s", syncPath)

			var syncPath = filepath.Join(syncPath, syncFileName)
			if syncMeta, err = ExtractSyncData(syncPath); err != nil {
				log.Fatal(err)
			}
			syncMeta.HomeDir = usr.HomeDir

			if err = ProcessSyncInfo(syncMeta); err != nil {
				log.Fatal(err)
			}
		}

		// log.Fatalf("%v", cmd.Flags().Args())
	},
}

func ExtractSyncData(path string) (SyncInfo, error) {
	var err error
	var info SyncInfo
	var syncFile *os.File
	var syncBytes []byte

	if syncFile, err = os.Open(path); err != nil {
		return info, fmt.Errorf("unable to open sync file: %s", path)
	}
	defer syncFile.Close()

	if syncBytes, err = io.ReadAll(syncFile); err != nil {
		log.Fatal("unable to read contents of sync file")
	}

	if err = json.Unmarshal([]byte(syncBytes), &info); err != nil {
		log.Fatal("`sync-pairs` flag contains invalid data")
	}
	if len(info.Data) == 0 {
		log.Fatal("no sync data found")
	}

	return info, err
}

func ProcessSyncInfo(info SyncInfo) error {
	var err error

	for _, data := range info.Data {
		for _, pair := range data.Pairs {
			pair.Source = strings.ReplaceAll(pair.Source, "~", info.HomeDir)
			pair.Target = strings.ReplaceAll(pair.Target, "~", info.HomeDir)

			if pair.Source, err = filepath.Abs(pair.Source); err != nil {
				log.Error(err)
				log.Fatalf("unable to determine absolute path: %s", pair.Source)
			}
			log.Infof("pair.Source = %s", pair.Source)

			if pair.Target, err = filepath.Abs(pair.Target); err != nil {
				log.Error(err)
				log.Fatalf("unable to determine absolute path: %s", pair.Target)
			}
			log.Infof("pair.Target = %s", pair.Target)

			if err = ProcessSyncPair(pair); err != nil {
				log.Fatal(err)
			}
		}
	}

	return err
}

func ProcessSyncPair(pair SyncPair) error {
	var md = ".md"

	return filepath.Walk(pair.Source, func(path string, info os.FileInfo, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		if info.IsDir() {
			return nil
		}

		var rel string
		if rel, errWalk = filepath.Rel(pair.Source, path); errWalk != nil {
			return errWalk
		}
		var dest = filepath.Join(pair.Target, rel)
		var ext = filepath.Ext(rel)
		var mtype = mime.TypeByExtension(ext)
		if ext == md {
			var fio *os.File
			if fio, errWalk = os.Open(path); errWalk != nil {
				log.Fatal(errWalk)
			}
			defer fio.Close()

			var buf = bufio.NewReader(fio)
			var matter = FrontMatter{}
			frontmatter.Parse(buf, &matter)
			if !matter.Draft {
				return ProcessMarkdown(path, dest)
			}
			return nil
		} else if len(mtype) >= 6 && mtype[:6] == "image/" {
			log.Debugf("mime = %s\t%s", mtype, rel)
			CopyFile(path, dest)
			return nil
		}

		return fmt.Errorf("unsupported file type [%s]: %s", mtype, path)
	})
}

func ProcessMarkdown(path string, dest string) error {
	/*
	https://help.obsidian.md/Editing+and+formatting/Callouts
	https://gohugo.io/content-management/shortcodes/
	https://discourse.gohugo.io/t/using-wide-with-markdown-images-and-hugo-processing/39879/9
	https://jpdroege.com/blog/hugo-shortcodes-partials/
	https://gohugo.io/content-management/shortcodes/
	*/
	var err error
	var fio *os.File
	var fdst *os.File
	var res pageparser.ContentFrontMatter

	if fio, err = os.Open(path); err != nil {
		log.Fatal(err)
	}
	defer fio.Close()

	if res, err = pageparser.ParseFrontMatterAndContent(fio); err != nil {
		log.Error(err)
	}
	var content = string(res.Content)

	var reCallouts = regexp.MustCompile(`(?s)\>\s*?\[!\w{2,10}\]\-?\+?.*?\n(?:\>.*?\n)+`)
	var cleaned = reCallouts.ReplaceAllStringFunc(content, func(dirty string) string {
		if len(dirty) == 0 {
			return ""
		}

		var callout strings.Builder
		for i, line := range strings.Split(dirty, "\n") {
			line = strings.TrimPrefix(line, "> ")
			if i == 0 {
				var open bool
				var parts = strings.SplitN(strings.TrimPrefix(line, "[!"), "]", 2)
				var cType = parts[0]
				var title = parts[1]
				if title[:2] == "+ " {
					open = true
					title = title[2:]
				} else if title[:2] == "- " {
					title = title[2:]
				}

				callout.WriteString(`{{% callout type="`)
				callout.WriteString(cType)
				callout.WriteString(`" `)

				if title != "" {
					callout.WriteString(`title="`)
					callout.WriteString(title)
					callout.WriteString(`" `)
				}

				if open {
					callout.WriteString(`open=true `)
				} else {
					// callout.WriteString(`open=false `)
				}

				callout.WriteString("%}}\n")
				continue
			}
			callout.WriteString(line)
			callout.WriteString("\n")
		}

		callout.WriteString("{{% /callout %}}\n")
		log.Debugf("callout=%s", callout.String())

		return callout.String()
	})

	var reLinks = regexp.MustCompile(`!?\[.*?\]\(.*?\)`)
	cleaned = reLinks.ReplaceAllStringFunc(cleaned, func(dirty string) string {
		if len(dirty) == 0 {
			return ""
		}

		var iAlt = strings.Index(dirty, "[") + 1
		var iAltEnd = iAlt + strings.Index(dirty[iAlt:], "]")
		if iAltEnd < iAlt {
			log.Fatalf("unknown issue: '%s'", dirty)
		}

		var iParams int
		if strings.Index(dirty[iAlt:iAltEnd], "|") < 0 {
			iParams = iAltEnd
		} else {
			iParams = iAlt + strings.Index(dirty[iAlt:iAltEnd], "|") + 1
		}
		var iAltTextEnd = iAltEnd
		if iAltEnd != iParams {
			iAltTextEnd = iParams - 1
		}

		var iLink = iAltEnd + strings.Index(dirty[iAltEnd:], "(") + 1
		if iLink < 0 {
			log.Fatalf("unknown issue: '%s'", dirty)
		}

		var iLinkEnd = iLink + strings.Index(dirty[iLink:], ")")
		if len(dirty[iLinkEnd+1:]) != 0 {
			log.Warnf(dirty[iLinkEnd+1:])
			log.Fatalf("extra link data = '%s'", dirty)
		}

		var img strings.Builder
		img.WriteString("{{% figure ")
		if len(dirty[iAlt:iAltTextEnd]) != 0 {
			img.WriteString("alt=\"")
			img.WriteString(dirty[iAlt:iAltTextEnd])
			img.WriteString("\" ")
			// img.WriteString("title=\"")
			// img.WriteString(dirty[iAlt:iAltTextEnd])
			// img.WriteString("\" ")
		}
		if len(dirty[iLink:iLinkEnd]) != 0 {
			img.WriteString("src=\"")
			img.WriteString(dirty[iLink:iLinkEnd])
			img.WriteString("\" ")
		}
		if len(dirty[iParams:iAltEnd]) != 0 {
			var extra = dirty[iParams:iAltEnd]
			var x = strings.Index(extra, "x")
			if x < 0 {
				img.WriteString("width=\"")
				img.WriteString(extra)
				img.WriteString("\" ")
			} else {
				img.WriteString("width=\"")
				img.WriteString(extra[:x])
				img.WriteString("\" ")
				if x+1 <= (len(extra)-1) && extra[x+1:] != "" {
					img.WriteString(", height=")
					img.WriteString(extra[x+1:])
					img.WriteString(" ")
				}
			}
		}
		img.WriteString("%}}")

		log.Debugf("result=%s", img.String())

		return img.String()
	})

	// save the processed content
	if fdst, err = os.OpenFile(dest, os.O_RDWR, 0755); err != nil {
		log.Fatal(err)
	}
	defer fdst.Close()

	var writeBuf bytes.Buffer
	if len(res.FrontMatter) != 0 {
		err := parser2.InterfaceToFrontMatter(res.FrontMatter, "yaml", &writeBuf)
		if err != nil {
			log.Error(err)
		}
	}
	writeBuf.WriteString(cleaned)
	fdst.Truncate(0)
	fdst.Seek(0,0)
	fdst.Write(writeBuf.Bytes())

	return err
}

func CopyFile(src string, dst string) error {
	var err error
	var srcStat os.FileInfo
	var source *os.File
	var destination *os.File

	if srcStat, err = os.Stat(src); err != nil {
		return err
	}
	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	if source, err = os.Open(src); err != nil {
		return err
	}
	defer source.Close()

	if destination, err = os.Create(dst); err != nil {
		return err
	}
	defer destination.Close()

	log.Debugf("copying %s", dst)

	log.Debugf("src = %s", src)
	log.Debugf("dst = %s", dst)
	if _, err = io.Copy(destination, source); err != nil {
		return err
	}
	if srcStat, err = os.Stat(src); err != nil {
		return err
	}
	if err = os.Chmod(dst, srcStat.Mode()); err != nil {
		return err
	}
	return nil
}



type Ext struct {
	Title string
	Description string
	DescriptionTag string
}

func (e *Ext) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(
		parser.WithASTTransformers(util.Prioritized(e, 999)),
	)
}

func (e* Ext) Transform(node *gast.Document, reader text.Reader, pc parser.Context) {
	tldrFound := false
	//node.Dump(reader.Source(), 0)
	for c :=  node.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == gast.KindHeading {
			h := c.(*gast.Heading)
			if h.Level == 1 {
				e.Title = string(c.Text(reader.Source()))
			} else if h.Level == 2 {
				t := c.FirstChild().(*gast.Text)
				h2Text := string(t.Text(reader.Source()))
				if h2Text == e.DescriptionTag {
					tldrFound = true
				}
			}
		}
		// Extract description
		if tldrFound && c.Kind() == gast.KindParagraph {
			log.Debug(c.FirstChild().Kind().String())
			e.Description = e.dumpStr(c, reader.Source(), "")
			return
		}
	}
}

func (e *Ext) dumpStr(c gast.Node, source []byte, str string) string {
	res := str
	for l := c.FirstChild(); l != nil; l = l.NextSibling() {
		if l.Kind() == gast.KindText {
			desc := string(l.Text(source))
			log.Debug(desc)
			res = res + desc
		} else {
			if l.HasChildren() {
				res = e.dumpStr(l, source, res)
			}
		}
	}
	return res
}



// Dir copies a whole directory recursively
func CopyDir(src string, dst string) error {

	log.Printf("Copy %s to %s\n\n", src, dst)

	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = CopyFile(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

func WalkMatch(root, pattern string) ([]string, error) {
	var matches []string
	var errWalk = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if true {
			log.Fatalf("%+v", info)
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if errWalk != nil {
		return nil, errWalk
	}
	return matches, nil
}
