package repository_test

import (
	"errors"
	"net/url"

	"github.com/cloudfoundry/cli/cf/v3/repository"

	authenticationfakes "github.com/cloudfoundry/cli/cf/api/authentication/fakes"
	ccClientFakes "github.com/cloudfoundry/go-ccapi/v3/client/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TokenRefresher", func() {
	var (
		ccClient       *ccClientFakes.FakeClient
		tokenRefresher *authenticationfakes.FakeTokenRefresher

		tokenHandler repository.TokenHandler
	)

	BeforeEach(func() {
		ccClient = &ccClientFakes.FakeClient{}
		tokenRefresher = &authenticationfakes.FakeTokenRefresher{}
		tokenHandler = repository.NewTokenHandler(ccClient, tokenRefresher)
	})

	Context("when the client returns an invalid token response", func() {
		BeforeEach(func() {
			ccClient.GetApplicationsReturns([]byte(`{"code":1000}`), nil)
		})

		It("tries to refresh the auth token", func() {
			tokenHandler.Do(func() ([]byte, error) {
				return ccClient.GetApplications(url.Values{})
			})

			Expect(tokenRefresher.RefreshAuthTokenCallCount()).To(Equal(1))
		})

		Context("when refreshing the auth token succeeds", func() {
			BeforeEach(func() {
				tokenRefresher.RefreshAuthTokenReturns("new-token", nil)
			})

			It("sets the auth token on the ccClient prior to executing the callback again", func() {
				getApplicationsCh := make(chan struct{})
				setTokenCh := make(chan struct{})

				ccClient.GetApplicationsStub = func(queryParams url.Values) ([]byte, error) {
					if ccClient.GetApplicationsCallCount() == 1 {
						return []byte(`{"code":1000}`), nil
					}

					getApplicationsCh <- struct{}{}

					return getApplicationsJSON, nil
				}

				ccClient.SetTokenStub = func(t string) {
					setTokenCh <- struct{}{}
				}

				go tokenHandler.Do(func() ([]byte, error) {
					return ccClient.GetApplications(url.Values{})
				})

				Consistently(getApplicationsCh).ShouldNot(Receive())
				Eventually(setTokenCh).Should(Receive())
				Eventually(getApplicationsCh).Should(Receive())

				Expect(ccClient.SetTokenArgsForCall(0)).To(Equal("new-token"))
			})
		})

		Context("when refreshing the auth token fails", func() {
			BeforeEach(func() {
				tokenRefresher.RefreshAuthTokenReturns("", errors.New("refresh-token-err"))
			})

			It("returns an error", func() {
				_, err := tokenHandler.Do(func() ([]byte, error) {
					return ccClient.GetApplications(url.Values{})
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to refresh auth token: refresh-token-err"))
			})
		})
	})

	Context("when the client does not return an invalid token response", func() {
		BeforeEach(func() {
			ccClient.GetApplicationsReturns([]byte("response"), errors.New("get-applications-err"))
		})

		It("does not try to refresh the auth token", func() {
			tokenHandler.Do(func() ([]byte, error) {
				return ccClient.GetApplications(url.Values{})
			})
			Expect(tokenRefresher.RefreshAuthTokenCallCount()).To(BeZero())
		})

		It("returns exactly what the callback returned", func() {
			response, err := tokenHandler.Do(func() ([]byte, error) {
				return ccClient.GetApplications(url.Values{})
			})
			Expect(response).To(Equal([]byte("response")))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("get-applications-err"))
		})
	})
})
