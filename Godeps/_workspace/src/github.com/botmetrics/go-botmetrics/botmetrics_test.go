package botmetrics

import (
	"net/http"
	"os"
	"testing"

	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/gomega/ghttp"
)

func TestContext(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "botmetrics")
}

var _ = Describe("botmetrics", func() {
	Describe("NewBotmetricsClient", func() {
		Context("with apiKey and botId", func() {
			It("should return a client and no error", func() {
				c, err := NewBotmetricsClient("api_key", "bot_id")
				Expect(c.ApiKey).To(Equal("api_key"))
				Expect(c.BotId).To(Equal("bot_id"))
				Expect(err).To(BeNil())
			})
		})

		Context("without apiKey and botId", func() {
			Context("without environment variables set", func() {
				It("should return an error", func() {
					c, err := NewBotmetricsClient()
					Expect(c).To(BeNil())
					Expect(err).To(Equal(ErrClientNotConfigured))
				})
			})

			Context("with environment variables set", func() {
				BeforeEach(func() {
					os.Setenv("BOTMETRICS_API_KEY", "api_key")
					os.Setenv("BOTMETRICS_BOT_ID", "bot_id")
				})

				AfterEach(func() {
					os.Unsetenv("BOTMETRICS_API_KEY")
					os.Unsetenv("BOTMETRICS_BOT_ID")
				})

				It("should return a client and no error", func() {
					c, err := NewBotmetricsClient()
					Expect(c.ApiKey).To(Equal("api_key"))
					Expect(c.BotId).To(Equal("bot_id"))
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("RegisterBot", func() {
		var server *ghttp.Server
		var bc *BotmetricsClient
		var err error

		BeforeEach(func() {
			server = ghttp.NewServer()
			os.Setenv("BOTMETRICS_API_HOST", server.URL())
			bc, err = NewBotmetricsClient("api-key", "bot-id")
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			server.Close()
			os.Unsetenv("BOTMETRICS_API_HOST")
		})

		verifyRequestResponse := func(server *ghttp.Server, method, uri, token, createdAt string, responseCode int64) {
			var verifier http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(method))
				Expect(r.RequestURI).To(Equal(uri))
				err := r.ParseForm()
				Expect(err).To(BeNil())
				Expect(r.Header.Get("Authorization")).To(Equal("api-key"))
				Expect(r.PostFormValue("instance[token]")).To(Equal(token))
				Expect(r.PostFormValue("instance[created_at]")).To(Equal(createdAt))
				Expect(r.PostFormValue("format")).To(Equal("json"))

				w.WriteHeader(int(responseCode))
			}

			server.AppendHandlers(verifier)
		}

		Context("successful API request", func() {
			Context("with token and no createdAt", func() {
				It("should return true", func() {
					verifyRequestResponse(server, "POST", "/bots/bot-id/instances", "bot-token", "", 201)
					status := bc.RegisterBot("bot-token", 0)
					Expect(status).To(BeTrue())
				})
			})

			Context("with token and createdAt", func() {
				It("should return true", func() {
					verifyRequestResponse(server, "POST", "/bots/bot-id/instances", "bot-token", "123456789", 201)
					status := bc.RegisterBot("bot-token", 123456789)
					Expect(status).To(BeTrue())
				})
			})
		})

		Context("unsuccessful API request", func() {
			It("should return true", func() {
				verifyRequestResponse(server, "POST", "/bots/bot-id/instances", "bot-token", "", 500)
				status := bc.RegisterBot("bot-token", 0)
				Expect(status).To(BeFalse())
			})
		})
	})
})
