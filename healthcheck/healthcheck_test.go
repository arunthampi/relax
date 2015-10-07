package healthcheck

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck")
}

var _ = Describe("HealthCheckServer", func() {
	var (
		hs           *HealthCheckServer
		frontend     *httptest.Server
		frontendIP   string
		frontendPort string
		err          error
	)

	Describe("Start", func() {
		BeforeEach(func() {
			hs = &HealthCheckServer{}

			frontend = httptest.NewServer(hs)
			frontendAddr := frontend.Listener.Addr().String()
			frontendIP, frontendPort, err = net.SplitHostPort(frontendAddr)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return with 200", func() {
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:%s", frontendIP, frontendPort), nil)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			Expect(strings.TrimRight(string(body), "\r\n")).To(Equal("relax alive"))
		})
	})
})
