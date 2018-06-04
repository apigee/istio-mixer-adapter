package apigee

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const proxiesPath = "apis"

// ProxiesService is an interface for interfacing with the Apigee Edge Admin API
// dealing with apiproxies.
type ProxiesService interface {
	List() ([]string, *Response, error)
	Get(string) (*Proxy, *Response, error)
	Import(string, string) (*ProxyRevision, *Response, error)
	Delete(string) (*DeletedProxyInfo, *Response, error)
	DeleteRevision(string, Revision) (*ProxyRevision, *Response, error)
	Deploy(string, string, Revision) (*ProxyRevisionDeployment, *Response, error)
	Undeploy(string, string, Revision) (*ProxyRevisionDeployment, *Response, error)
	Export(string, Revision) (string, *Response, error)
	GetDeployments(string) (*ProxyDeployment, *Response, error)
	GetDeployedRevision(proxy, env string) (*Revision, error)
}

type ProxiesServiceOp struct {
	client *EdgeClient
}

var _ ProxiesService = &ProxiesServiceOp{}

// Proxy contains information about an API Proxy within an Edge organization.
type Proxy struct {
	Revisions []Revision    `json:"revision,omitempty"`
	Name      string        `json:"name,omitempty"`
	MetaData  ProxyMetadata `json:"metaData,omitempty"`
}

// ProxyMetadata contains information related to the creation and last modified
// time and actor for an API Proxy within an organization.
type ProxyMetadata struct {
	LastModifiedBy string    `json:"lastModifiedBy,omitempty"`
	CreatedBy      string    `json:"createdBy,omitempty"`
	LastModifiedAt Timestamp `json:"lastModifiedAt,omitempty"`
	CreatedAt      Timestamp `json:"createdAt,omitempty"`
}

// ProxyRevision holds information about a revision of an API Proxy.
type ProxyRevision struct {
	CreatedBy       string    `json:"createdBy,omitempty"`
	CreatedAt       Timestamp `json:"createdAt,omitempty"`
	Description     string    `json:"description,omitempty"`
	ContextInfo     string    `json:"contextInfo,omitempty"`
	DisplayName     string    `json:"displayName,omitempty"`
	Name            string    `json:"name,omitempty"`
	LastModifiedBy  string    `json:"lastModifiedBy,omitempty"`
	LastModifiedAt  Timestamp `json:"lastModifiedAt,omitempty"`
	Revision        Revision  `json:"revision,omitempty"`
	TargetEndpoints []string  `json:"targetEndpoints,omitempty"`
	TargetServers   []string  `json:"targetServers,omitempty"`
	Resources       []string  `json:"resources,omitempty"`
	ProxyEndpoints  []string  `json:"proxyEndpoints,omitempty"`
	Policies        []string  `json:"policies,omitempty"`
	Type            string    `json:"type,omitempty"`
}

// ProxyRevisionDeployment holds information about the deployment state of a
// single revision of an API Proxy.
type ProxyRevisionDeployment struct {
	Name         string       `json:"aPIProxy,omitempty"`
	Revision     Revision     `json:"revision,omitempty"`
	Environment  string       `json:"environment,omitempty"`
	Organization string       `json:"organization,omitempty"`
	State        string       `json:"state,omitempty"`
	Servers      []EdgeServer `json:"server,omitempty"`
}

// When inquiring the deployment status of an API PRoxy revision, even implicitly
// as when performing a Deploy or Undeploy, the response includes the deployment
// status for each particular Edge Server in the environment. This struct
// deserializes that information. It will normally not be useful at all. In rare
// cases, it may be useful in helping to diagnose problems.  For example, if there
// is a problem with a deployment change, as when a Message Processor is
// experiencing a problem and cannot undeploy, or more commonly, cannot deploy an
// API Proxy, this struct will hold relevant information.
type EdgeServer struct {
	Status string   `json:"status,omitempty"`
	Uuid   string   `json:"uUID,omitempty"`
	Type   []string `json:"type,omitempty"`
}

// ProxyDeployment holds information about the deployment state of a
// all revisions of an API Proxy.
type ProxyDeployment struct {
	Environments []EnvironmentDeployment `json:"environment,omitempty"`
	Name         string                  `json:"name,omitempty"`
	Organization string                  `json:"organization,omitempty"`
}

type EnvironmentDeployment struct {
	Name     string               `json:"name,omitempty"`
	Revision []RevisionDeployment `json:"revision,omitempty"`
}

type RevisionDeployment struct {
	Number  Revision     `json:"name,omitempty"`
	State   string       `json:"state,omitempty"`
	Servers []EdgeServer `json:"server,omitempty"`
}

// When Delete returns successfully, it returns a payload that contains very little useful
// information. This struct deserializes that information.
type DeletedProxyInfo struct {
	Name string `json:"name,omitempty"`
}

// type proxiesRoot struct {
//   Proxies []Proxy `json:"proxies"`
// }

// List retrieves the list of apiproxy names for the organization referred by the EdgeClient.
func (s *ProxiesServiceOp) List() ([]string, *Response, error) {
	req, e := s.client.NewRequest("GET", proxiesPath, nil)
	if e != nil {
		return nil, nil, e
	}
	namelist := make([]string, 0)
	resp, e := s.client.Do(req, &namelist)
	if e != nil {
		return nil, resp, e
	}
	return namelist, resp, e
}

// Get retrieves the information about an API Proxy in an organization, information including
// the list of available revisions, and the created and last modified dates and actors.
func (s *ProxiesServiceOp) Get(proxy string) (*Proxy, *Response, error) {
	path := path.Join(proxiesPath, proxy)
	req, e := s.client.NewRequest("GET", path, nil)
	if e != nil {
		return nil, nil, e
	}
	returnedProxy := Proxy{}
	resp, e := s.client.Do(req, &returnedProxy)
	if e != nil {
		return nil, resp, e
	}
	return &returnedProxy, resp, e
}

func smartFilter(path string) bool {
	if strings.HasSuffix(path, "~") {
		return false
	}
	if strings.HasSuffix(path, "#") && strings.HasPrefix(path, "#") {
		return false
	}
	return true
}

func zipDirectory(source string, target string, filter func(string) bool) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if filter == nil || filter(path) {
			if err != nil {
				return err
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			}

			// This archive will be unzipped by a Java process.  When ZIP64 extensions
			// are used, Java insists on having Deflate as the compression method (0x08)
			// even for directories.
			header.Method = zip.Deflate

			if info.IsDir() {
				header.Name += "/"
			}

			writer, err := archive.CreateHeader(header)
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
		}
		return err
	})

	return err
}

// Import an API proxy into an organization, creating a new API Proxy revision.
// The proxyName can be passed as "nil" in which case the name is derived from the source.
// The source can be either a filesystem directory containing an exploded apiproxy bundle, OR
// the path of a zip file containing an API Proxy bundle. Returns the API proxy revision information.
// This method does not deploy the imported proxy. See the Deploy method.
func (s *ProxiesServiceOp) Import(proxyName string, source string) (*ProxyRevision, *Response, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, nil, err
	}
	zipfileName := source
	if info.IsDir() {
		// create a temporary zip file
		if proxyName == "" {
			proxyName = filepath.Base(source)
		}
		tempDir, e := ioutil.TempDir("", "go-apigee-edge-")
		if e != nil {
			return nil, nil, errors.New(fmt.Sprintf("while creating temp dir, error: %#v", e))
		}
		zipfileName = path.Join(tempDir, "apiproxy.zip")
		e = zipDirectory(path.Join(source, "apiproxy"), zipfileName, smartFilter)
		if e != nil {
			return nil, nil, errors.New(fmt.Sprintf("while creating temp dir, error: %#v", e))
		}
		//fmt.Printf("zipped %s into %s\n\n", source, zipfileName)
	}

	if !strings.HasSuffix(zipfileName, ".zip") {
		return nil, nil, errors.New("source must be a zipfile")
	}

	info, err = os.Stat(zipfileName)
	if err != nil {
		return nil, nil, err
	}

	// append the query params
	origURL, err := url.Parse(proxiesPath)
	if err != nil {
		return nil, nil, err
	}
	q := origURL.Query()
	q.Add("action", "import")
	q.Add("name", proxyName)
	origURL.RawQuery = q.Encode()
	path := origURL.String()

	ioreader, err := os.Open(zipfileName)
	if err != nil {
		return nil, nil, err
	}
	defer ioreader.Close()

	req, e := s.client.NewRequest("POST", path, ioreader)
	if e != nil {
		return nil, nil, e
	}
	returnedProxyRevision := ProxyRevision{}
	resp, e := s.client.Do(req, &returnedProxyRevision)
	if e != nil {
		return nil, resp, e
	}
	return &returnedProxyRevision, resp, e
}

// Export a revision of an API proxy within an organization, to a filesystem file.
func (s *ProxiesServiceOp) Export(proxyName string, rev Revision) (string, *Response, error) {
	// curl -u USER:PASSWORD \
	//  http://MGMTSERVER/v1/o/ORGNAME/apis/APINAME/revisions/REVNUMBER?format=bundle > bundle.zip

	path := path.Join(proxiesPath, proxyName, "revisions", fmt.Sprintf("%d", rev))
	// append the required query param
	origURL, err := url.Parse(path)
	if err != nil {
		return "", nil, err
	}
	q := origURL.Query()
	q.Add("format", "bundle")
	origURL.RawQuery = q.Encode()
	path = origURL.String()

	req, e := s.client.NewRequest("GET", path, nil)
	if e != nil {
		return "", nil, e
	}
	req.Header.Del("Accept")

	t := time.Now()
	filename := fmt.Sprintf("proxyName-r%d-%d%02d%02d-%02d%02d%02d.zip",
		rev, t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())

	out, e := os.Create(filename)
	if e != nil {
		return "", nil, e
	}

	resp, e := s.client.Do(req, out)
	if e != nil {
		return "", resp, e
	}
	out.Close()
	return filename, resp, e
}

// DeleteRevision deletes a specific revision of an API Proxy from an organization.
// The revision must exist, and must not be currently deployed.
func (s *ProxiesServiceOp) DeleteRevision(proxyName string, rev Revision) (*ProxyRevision, *Response, error) {
	path := path.Join(proxiesPath, proxyName, "revisions", fmt.Sprintf("%d", rev))
	req, e := s.client.NewRequest("DELETE", path, nil)
	if e != nil {
		return nil, nil, e
	}
	proxyRev := ProxyRevision{}
	resp, e := s.client.Do(req, &proxyRev)
	if e != nil {
		return nil, resp, e
	}
	return &proxyRev, resp, e
}

// Undeploy a specific revision of an API Proxy from a particular environment within an Edge organization.
func (s *ProxiesServiceOp) Undeploy(proxyName, env string, rev Revision) (*ProxyRevisionDeployment, *Response, error) {
	path := path.Join(proxiesPath, proxyName, "revisions", fmt.Sprintf("%d", rev), "deployments")
	// append the query params
	origURL, err := url.Parse(path)
	if err != nil {
		return nil, nil, err
	}
	q := origURL.Query()
	q.Add("action", "undeploy")
	q.Add("env", env)
	origURL.RawQuery = q.Encode()
	path = origURL.String()

	req, e := s.client.NewRequest("POST", path, nil)
	if e != nil {
		return nil, nil, e
	}

	deployment := ProxyRevisionDeployment{}
	resp, e := s.client.Do(req, &deployment)
	if e != nil {
		return nil, resp, e
	}
	return &deployment, resp, e
}

// Deploy a revision of an API proxy to a specific environment within an organization.
func (s *ProxiesServiceOp) Deploy(proxyName, env string, rev Revision) (*ProxyRevisionDeployment, *Response, error) {
	path := path.Join(proxiesPath, proxyName, "revisions", fmt.Sprintf("%d", rev), "deployments")
	// append the query params
	origURL, err := url.Parse(path)
	if err != nil {
		return nil, nil, err
	}
	q := origURL.Query()
	q.Add("action", "deploy")
	q.Add("override", "true")
	q.Add("delay", "12")
	q.Add("env", env)
	origURL.RawQuery = q.Encode()
	path = origURL.String()

	req, e := s.client.NewRequest("POST", path, nil)
	if e != nil {
		return nil, nil, e
	}

	deployment := ProxyRevisionDeployment{}
	resp, e := s.client.Do(req, &deployment)
	if e != nil {
		return nil, resp, e
	}
	return &deployment, resp, e
}

// Delete an API Proxy and all its revisions from an organization. This method
// will fail if any of the revisions of the named API Proxy are currently deployed
// in any environment.
func (s *ProxiesServiceOp) Delete(proxyName string) (*DeletedProxyInfo, *Response, error) {
	path := path.Join(proxiesPath, proxyName)
	req, e := s.client.NewRequest("DELETE", path, nil)
	if e != nil {
		return nil, nil, e
	}
	proxy := DeletedProxyInfo{}
	resp, e := s.client.Do(req, &proxy)
	if e != nil {
		return nil, resp, e
	}
	return &proxy, resp, e
}

// GetDeployments retrieves the information about deployments of an API Proxy in
// an organization, including the environment names and revision numbers.
func (s *ProxiesServiceOp) GetDeployments(proxy string) (*ProxyDeployment, *Response, error) {
	path := path.Join(proxiesPath, proxy, "deployments")
	req, e := s.client.NewRequest("GET", path, nil)
	if e != nil {
		return nil, nil, e
	}
	deployments := ProxyDeployment{}
	resp, e := s.client.Do(req, &deployments)
	if e != nil {
		return nil, resp, e
	}
	return &deployments, resp, e
}

func (s *ProxiesServiceOp) GetDeployedRevision(proxy, env string) (*Revision, error) {
	deployment, resp, err := s.GetDeployments(proxy)
	if err != nil && resp == nil {
		return nil, err
	}
	if deployment != nil {
		if resp.StatusCode != 404 {
			for _, ed := range deployment.Environments {
				if ed.Name == env {
					for _, rev := range ed.Revision {
						if rev.State == "deployed" {
							return &ed.Revision[0].Number, nil
						}
					}
				}
			}
		}
	}

	return nil, nil
}
