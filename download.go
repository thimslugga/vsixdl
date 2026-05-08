package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadCmd handles single and batch downloads.
//
// Examples:
//
//	vsixdl get ms-python.python
//	vsixdl get ms-python.python golang.go rust-lang.rust-analyzer
//	vsixdl get --version 2024.22.0 ms-python.python
//	vsixdl --source openvsx get golang.go
//	vsixdl get --from-file extensions.txt --output ./bundle
type DownloadCmd struct {
	Extensions     []string `arg:"" optional:"" help:"One or more publisher.name identifiers."`
	FromFile       string   `short:"f" name:"from-file" help:"Read extension identifiers from a file (one per line, # comments allowed)." type:"existingfile"`
	Version        string   `short:"v" default:"latest"  help:"Version to download. Ignored in batch mode."`
	Output         string   `short:"o" default:"."        help:"Output directory."`
	TargetPlatform string   `name:"target-platform"      help:"Target platform (linux-x64, win32-x64, darwin-arm64, ...). Empty = universal."`
	PreRelease     bool     `name:"pre-release"           help:"Allow pre-release versions when resolving 'latest'."`
	Force          bool     `help:"Overwrite existing files instead of skipping."`
}

func (c *DownloadCmd) Run(cli *CLI) error {
	items, err := c.collectExtensions()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no extensions specified (provide extension identifiers or use --from-file)")
	}

	if err := os.MkdirAll(c.Output, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	client := newClient(cli.Source)

	var failed int
	for _, item := range items {
		if err := c.fetchOne(cli, client, item); err != nil {
			fmt.Fprintf(os.Stderr, "failed: %s: %v\n", item.id, err)
			failed++
			continue
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d downloads failed", failed, len(items))
	}
	return nil
}

// extItem is one parsed entry from the CLI args or batch file.
type extItem struct {
	id      string // original identifier as written
	pub     string
	name    string
	version string // empty means use cmd default
}

func (c *DownloadCmd) collectExtensions() ([]extItem, error) {
	var raw []string
	raw = append(raw, c.Extensions...)

	if c.FromFile != "" {
		fileItems, err := readExtFile(c.FromFile)
		if err != nil {
			return nil, err
		}
		raw = append(raw, fileItems...)
	}

	out := make([]extItem, 0, len(raw))
	for _, r := range raw {
		// Allow "publisher.name@version" overrides in batch lists.
		id, ver := r, ""
		if at := strings.Index(r, "@"); at != -1 {
			id, ver = r[:at], r[at+1:]
		}
		pub, name, err := parseExt(id)
		if err != nil {
			return nil, err
		}
		out = append(out, extItem{id: id, pub: pub, name: name, version: ver})
	}
	return out, nil
}

// readExtFile reads a list file: one identifier per line, blank lines and
// '#' comments ignored. Versions can be pinned with '@version' suffix.
func readExtFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip trailing inline comments.
		if hash := strings.Index(line, " #"); hash != -1 {
			line = strings.TrimSpace(line[:hash])
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func (c *DownloadCmd) fetchOne(cli *CLI, client registry, item extItem) error {
	want := item.version
	if want == "" {
		want = c.Version
	}

	resolved, url, err := client.resolve(item.pub, item.name, want, c.TargetPlatform, c.PreRelease)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.%s-%s.vsix", item.pub, item.name, resolved)
	if c.TargetPlatform != "" {
		filename = fmt.Sprintf("%s.%s-%s@%s.vsix", item.pub, item.name, resolved, c.TargetPlatform)
	}
	outPath := filepath.Join(c.Output, filename)

	if !c.Force {
		if _, err := os.Stat(outPath); err == nil {
			if !cli.Quiet {
				fmt.Fprintf(os.Stderr, "skip %s (exists)\n", outPath)
			}
			fmt.Println(outPath)
			return nil
		}
	}

	if !cli.Quiet {
		fmt.Fprintf(os.Stderr, "fetch %s %s [%s]\n", item.id, resolved, cli.Source)
	}
	if err := downloadFile(url, outPath, cli.Quiet); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

// downloadFile streams url to dest atomically (temp file then rename).
// Shows a simple percentage meter on stderr unless quiet.
func downloadFile(url, dest string, quiet bool) error {
	tmp := dest + ".tmp"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vsixdl/"+version)

	hc := &http.Client{Timeout: 5 * time.Minute}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	var src io.Reader = resp.Body
	if !quiet && resp.ContentLength > 0 {
		src = &progressReader{r: resp.Body, total: resp.ContentLength}
	}

	if _, copyErr := io.Copy(f, src); copyErr != nil {
		f.Close()
		os.Remove(tmp)
		return copyErr
	}
	if closeErr := f.Close(); closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	if !quiet && resp.ContentLength > 0 {
		fmt.Fprintln(os.Stderr) // newline after the progress meter
	}

	return os.Rename(tmp, dest)
}

// progressReader prints a percentage to stderr as bytes flow through.
type progressReader struct {
	r       io.Reader
	total   int64
	read    int64
	lastPct int
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	pct := int(p.read * 100 / p.total)
	if pct != p.lastPct {
		fmt.Fprintf(os.Stderr, "\r  %3d%% (%d / %d bytes)", pct, p.read, p.total)
		p.lastPct = pct
	}
	return n, err
}
