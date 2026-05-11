package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// registry is the contract every backend implements.
// Two impls: MS Marketplace and Open VSX.
type registry interface {
	resolve(publisher, name, version, targetPlatform string, preRelease bool) (resolvedVersion, downloadURL string, err error)
	info(publisher, name string) (*extensionInfo, error)
	versions(publisher, name string) ([]versionInfo, error)
}

type extensionInfo struct {
	DisplayName string
	Description string
	Publisher   string
	Name        string
	Latest      string
	Versions    []versionInfo
}

type versionInfo struct {
	Version        string
	TargetPlatform string
	Date           string
	PreRelease     bool
}

func newClient(source string) registry {
	switch source {
	case "openvsx":
		return &openVSXClient{base: "https://open-vsx.org/api"}
	default:
		return &msMarketplaceClient{base: "https://marketplace.visualstudio.com/_apis/public/gallery"}
	}
}

// parseExt splits "publisher.name" into its parts. Publishers themselves
// can contain dashes but never dots, so a simple SplitN on the first dot
// is correct.
func parseExt(s string) (publisher, name string, err error) {
	idx := strings.Index(s, ".")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", fmt.Errorf("extension must be in publisher.name format, got %q", s)
	}
	return s[:idx], s[idx+1:], nil
}

// MS Marketplace API client
type msMarketplaceClient struct {
	base string
}

const mpAPIVersion = "3.0-preview.1"

// MS Marketplace flag bits. The numeric values come from MS gallery API
// and are stable. We OR them together to ask for the data we need.
const (
	mpFlagIncludeVersions          = 0x1
	mpFlagIncludeFiles             = 0x2
	mpFlagIncludeAssetURI          = 0x80
	mpFlagIncludeStatistics        = 0x100
	mpFlagExcludeNonValidated      = 0x20
	mpFlagIncludeVersionProperties = 0x400
)

// Filter type 7 = "ExtensionName" (the publisher.name form).
const mpFilterExtensionName = 7

type mpQueryRequest struct {
	Filters []mpFilter `json:"filters"`
	Flags   int        `json:"flags"`
}

type mpFilter struct {
	Criteria []mpCriterion `json:"criteria"`
}

type mpCriterion struct {
	FilterType int    `json:"filterType"`
	Value      string `json:"value"`
}

type mpQueryResponse struct {
	Results []struct {
		Extensions []mpExtension `json:"extensions"`
	} `json:"results"`
}

type mpExtension struct {
	ExtensionName    string `json:"extensionName"`
	DisplayName      string `json:"displayName"`
	ShortDescription string `json:"shortDescription"`
	Publisher        struct {
		PublisherName string `json:"publisherName"`
	} `json:"publisher"`
	Versions []mpVersion `json:"versions"`
}

type mpVersion struct {
	Version        string       `json:"version"`
	TargetPlatform string       `json:"targetPlatform,omitempty"`
	LastUpdated    string       `json:"lastUpdated"`
	Properties     []mpProperty `json:"properties"`
	AssetURI       string       `json:"assetUri"`
}

type mpProperty struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (m *msMarketplaceClient) query(itemName string, flags int) (*mpExtension, error) {
	body := mpQueryRequest{
		Filters: []mpFilter{
			{Criteria: []mpCriterion{
				{FilterType: mpFilterExtensionName, Value: itemName},
			}},
		},
		Flags: flags,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, m.base+"/extensionquery", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json;api-version="+mpAPIVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "vsixdl/"+version)

	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ms marketplace query failed: %s", resp.Status)
	}

	var out mpQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Results) == 0 || len(out.Results[0].Extensions) == 0 {
		return nil, fmt.Errorf("extension not found: %s", itemName)
	}
	return &out.Results[0].Extensions[0], nil
}

func (m *msMarketplaceClient) resolve(publisher, name, want, targetPlatform string, preRelease bool) (string, string, error) {
	itemName := publisher + "." + name
	flags := mpFlagIncludeVersions | mpFlagIncludeFiles | mpFlagIncludeAssetURI |
		mpFlagIncludeVersionProperties | mpFlagExcludeNonValidated
	ext, err := m.query(itemName, flags)
	if err != nil {
		return "", "", err
	}

	pick, err := pickMPVersion(ext.Versions, want, targetPlatform, preRelease)
	if err != nil {
		return "", "", err
	}

	// The asset URI is the per-version base. Append the well-known asset
	// type for the .vsix package itself.
	url := pick.AssetURI + "/Microsoft.VisualStudio.Services.VSIXPackage"
	if pick.TargetPlatform != "" {
		url += "?targetPlatform=" + pick.TargetPlatform
	}
	return pick.Version, url, nil
}

// pickMPVersion picks the right entry from the MS marketplace's version list.
// Versions are returned newest-first by the API, so we just walk them.
func pickMPVersion(versions []mpVersion, want, targetPlatform string, preRelease bool) (*mpVersion, error) {
	for i := range versions {
		v := &versions[i]
		// Platform filter. If a target was requested, require an exact
		// match. If no target was requested, only accept platform-neutral
		// entries (TargetPlatform == "").
		if targetPlatform != "" {
			if v.TargetPlatform != targetPlatform {
				continue
			}
		} else if v.TargetPlatform != "" {
			continue
		}
		if !preRelease && isMPPreRelease(v) {
			continue
		}
		if want == "" || want == "latest" {
			return v, nil
		}
		if v.Version == want {
			return v, nil
		}
	}
	return nil, fmt.Errorf("no matching version found (wanted=%q target=%q)", want, targetPlatform)
}

func isMPPreRelease(v *mpVersion) bool {
	for _, p := range v.Properties {
		if p.Key == "Microsoft.VisualStudio.Code.PreRelease" && p.Value == "true" {
			return true
		}
	}
	return false
}

func (m *msMarketplaceClient) info(publisher, name string) (*extensionInfo, error) {
	itemName := publisher + "." + name
	flags := mpFlagIncludeVersions | mpFlagIncludeStatistics | mpFlagIncludeVersionProperties
	ext, err := m.query(itemName, flags)
	if err != nil {
		return nil, err
	}
	info := &extensionInfo{
		DisplayName: ext.DisplayName,
		Description: ext.ShortDescription,
		Publisher:   ext.Publisher.PublisherName,
		Name:        ext.ExtensionName,
	}
	for i := range ext.Versions {
		v := &ext.Versions[i]
		pre := isMPPreRelease(v)
		if info.Latest == "" && v.TargetPlatform == "" && !pre {
			info.Latest = v.Version
		}
		info.Versions = append(info.Versions, versionInfo{
			Version:        v.Version,
			TargetPlatform: v.TargetPlatform,
			Date:           v.LastUpdated,
			PreRelease:     pre,
		})
	}
	return info, nil
}

func (m *msMarketplaceClient) versions(publisher, name string) ([]versionInfo, error) {
	info, err := m.info(publisher, name)
	if err != nil {
		return nil, err
	}
	return info.Versions, nil
}

// Open VSX client

type openVSXClient struct {
	base string
}

type ovsxExtension struct {
	Namespace      string            `json:"namespace"`
	Name           string            `json:"name"`
	DisplayName    string            `json:"displayName"`
	Description    string            `json:"description"`
	Version        string            `json:"version"`
	TargetPlatform string            `json:"targetPlatform"`
	PreRelease     bool              `json:"preRelease"`
	Timestamp      string            `json:"timestamp"`
	Files          map[string]string `json:"files"`
	AllVersions    map[string]string `json:"allVersions"`
}

func (o *openVSXClient) get(path string, dst interface{}) error {
	req, err := http.NewRequest(http.MethodGet, o.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vsixdl/"+version)

	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(resp.Body).Decode(dst)
	case http.StatusNotFound:
		return fmt.Errorf("not found: %s", path)
	default:
		return fmt.Errorf("openvsx: %s", resp.Status)
	}
}

func (o *openVSXClient) resolve(publisher, name, want, targetPlatform string, preRelease bool) (string, string, error) {
	// Open VSX path layout:
	//   /{namespace}/{extension}                            -> latest universal
	//   /{namespace}/{extension}/{version}                  -> specific universal
	//   /{namespace}/{extension}/{targetPlatform}           -> latest platform
	//   /{namespace}/{extension}/{targetPlatform}/{version} -> specific platform
	path := "/" + publisher + "/" + name
	if targetPlatform != "" {
		path += "/" + targetPlatform
	}
	if want != "" && want != "latest" {
		path += "/" + want
	}

	var ext ovsxExtension
	if err := o.get(path, &ext); err != nil {
		return "", "", err
	}

	if !preRelease && ext.PreRelease && (want == "" || want == "latest") {
		// Open VSX's "latest" alias may include pre-releases. Without an
		// explicit version pin, warn the caller via error.
		return "", "", fmt.Errorf("latest %s.%s is a pre-release; pass --pre-release or pin --version", publisher, name)
	}

	dl := ext.Files["download"]
	if dl == "" {
		return "", "", fmt.Errorf("no download URL in openvsx response")
	}
	return ext.Version, dl, nil
}

func (o *openVSXClient) info(publisher, name string) (*extensionInfo, error) {
	var ext ovsxExtension
	if err := o.get("/"+publisher+"/"+name, &ext); err != nil {
		return nil, err
	}
	info := &extensionInfo{
		DisplayName: ext.DisplayName,
		Description: ext.Description,
		Publisher:   ext.Namespace,
		Name:        ext.Name,
		Latest:      ext.Version,
	}
	for v := range ext.AllVersions {
		info.Versions = append(info.Versions, versionInfo{Version: v})
	}
	return info, nil
}

func (o *openVSXClient) versions(publisher, name string) ([]versionInfo, error) {
	info, err := o.info(publisher, name)
	if err != nil {
		return nil, err
	}
	return info.Versions, nil
}
