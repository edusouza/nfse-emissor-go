package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// repoURL is where a link goes when the file it points to is not a page of
// the site: the source code, the annexes, the examples.
const repoURL = "https://github.com/edusouza/nfse-emissor-go"

// generatedMarker opens the comment at the top of every page this program
// writes. The documentation tests in internal/docs look for it to tell the
// pages written by hand from the ones generated here.
const generatedMarker = "<!-- Gerada por internal/sitegen"

const (
	decisionsDir  = "docs/decisoes"
	decisionsPage = "decisoes"
	changelogFile = "CHANGELOG.md"
	changelogPage = "changelog.md"
)

// writeDecisions copies every ADR to the site, with its links fixed.
//
// The directory is emptied first: it holds nothing but copies, and an ADR
// renamed in the repository would otherwise stay published under its old
// name.
func writeDecisions(root, out string) error {
	src := filepath.Join(root, filepath.FromSlash(decisionsDir))
	files, err := filepath.Glob(filepath.Join(src, "*.md"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("nenhuma ADR em %s", src)
	}

	dst := filepath.Join(out, decisionsPage)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	// #nosec G301 -- the generated site is public and read by the site builder
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	for _, f := range files {
		from := decisionsDir + "/" + filepath.Base(f)
		to, _ := sitePage(from)
		if err := copyPage(root, out, from, to); err != nil {
			return err
		}
	}
	fmt.Printf("%s: %d paginas\n", dst, len(files))
	return nil
}

func writeChangelog(root, out string) error {
	return copyPage(root, out, changelogFile, changelogPage)
}

// copyPage copies a Markdown file of the repository to the site. Both paths
// are slash-separated: from relative to the repository, to relative to the
// site content.
func copyPage(root, out, from, to string) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from))) // #nosec G304 -- reads the repository's own sources
	if err != nil {
		return err
	}
	isDir := func(p string) bool {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))) // #nosec G703 -- a link target in the repository's own Markdown
		return err == nil && info.IsDir()
	}
	text := fmt.Sprintf("%s a partir de %s. Edite o original. -->\n\n", generatedMarker, from) +
		rewriteLinks(string(data), from, to, isDir)

	dst := filepath.Join(out, filepath.FromSlash(to))
	// #nosec G301 -- the generated site is public and read by the site builder
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(text), 0o644) // #nosec G306 G703 -- a public page, at a path built from the repository's own sources
}

// sitePage tells where a file of the repository is published on the site, if
// it is.
func sitePage(repoPath string) (string, bool) {
	switch {
	case repoPath == changelogFile:
		return changelogPage, true
	case repoPath == decisionsDir+"/README.md":
		return decisionsPage + "/index.md", true
	case path.Dir(repoPath) == decisionsDir && path.Ext(repoPath) == ".md":
		return decisionsPage + "/" + path.Base(repoPath), true
	}
	return "", false
}

var (
	inlineLink = regexp.MustCompile(`(\]\()([^)\s]+)((?:\s+"[^"]*")?\))`)
	refLink    = regexp.MustCompile(`^(\s{0,3}\[[^\]]+\]:\s*)(\S+)(.*)$`)
	fence      = regexp.MustCompile("^\\s*(```|~~~)")
	scheme     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// rewriteLinks makes the relative links of a file hold where the site puts
// it. A link to another published page becomes a link between pages; a link
// to anything else in the repository goes to GitHub, since the site does not
// carry the source. from is the file's path in the repository, to its path
// in the site; both are slash-separated.
func rewriteLinks(text, from, to string, isDir func(string) bool) string {
	target := func(t string) string {
		if strings.HasPrefix(t, "#") || scheme.MatchString(t) {
			return t
		}
		p, frag, _ := strings.Cut(t, "#")
		if frag != "" {
			frag = "#" + frag
		}
		resolved := path.Clean(path.Join(path.Dir(from), p))
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			return t
		}
		if page, ok := sitePage(resolved); ok {
			return relative(path.Dir(to), page) + frag
		}
		kind := "blob"
		if isDir(resolved) {
			kind = "tree"
		}
		return repoURL + "/" + kind + "/master/" + resolved + frag
	}

	lines := strings.Split(text, "\n")
	inFence := false
	for i, line := range lines {
		if fence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := refLink.FindStringSubmatch(line); m != nil {
			lines[i] = m[1] + target(m[2]) + m[3]
			continue
		}
		lines[i] = inlineLink.ReplaceAllStringFunc(line, func(s string) string {
			m := inlineLink.FindStringSubmatch(s)
			return m[1] + target(m[2]) + m[3]
		})
	}
	return strings.Join(lines, "\n")
}

// relative is the slash-separated path from directory dir to file target,
// both relative to the same root.
func relative(dir, target string) string {
	rel, err := filepath.Rel(filepath.FromSlash(dir), filepath.FromSlash(target))
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

// githubBlob is the address of a repository file on GitHub.
func githubBlob(repoPath string) string {
	return repoURL + "/blob/master/" + repoPath
}
