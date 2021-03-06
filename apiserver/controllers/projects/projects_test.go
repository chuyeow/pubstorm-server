package projects_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/nitrous-io/rise-server/apiserver/common"
	"github.com/nitrous-io/rise-server/apiserver/dbconn"
	"github.com/nitrous-io/rise-server/apiserver/models/cert"
	"github.com/nitrous-io/rise-server/apiserver/models/deployment"
	"github.com/nitrous-io/rise-server/apiserver/models/domain"
	"github.com/nitrous-io/rise-server/apiserver/models/oauthtoken"
	"github.com/nitrous-io/rise-server/apiserver/models/project"
	"github.com/nitrous-io/rise-server/apiserver/models/rawbundle"
	"github.com/nitrous-io/rise-server/apiserver/models/user"
	"github.com/nitrous-io/rise-server/apiserver/server"
	"github.com/nitrous-io/rise-server/pkg/filetransfer"
	"github.com/nitrous-io/rise-server/pkg/mqconn"
	"github.com/nitrous-io/rise-server/pkg/tracker"
	"github.com/nitrous-io/rise-server/shared"
	"github.com/nitrous-io/rise-server/shared/exchanges"
	"github.com/nitrous-io/rise-server/shared/queues"
	"github.com/nitrous-io/rise-server/shared/s3client"
	"github.com/nitrous-io/rise-server/testhelper"
	"github.com/nitrous-io/rise-server/testhelper/factories"
	"github.com/nitrous-io/rise-server/testhelper/fake"
	"github.com/nitrous-io/rise-server/testhelper/sharedexamples"
	"github.com/streadway/amqp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "projects")
}

var _ = Describe("Projects", func() {
	var (
		db  *gorm.DB
		s   *httptest.Server
		res *http.Response
		err error

		u *user.User
		t *oauthtoken.OauthToken

		fakeTracker *fake.Tracker
		origTracker tracker.Trackable
	)

	BeforeEach(func() {
		db, err = dbconn.DB()
		Expect(err).To(BeNil())
		testhelper.TruncateTables(db.DB())

		u, _, t = factories.AuthTrio(db)

		origTracker = common.Tracker
		fakeTracker = &fake.Tracker{}
		common.Tracker = fakeTracker
	})

	AfterEach(func() {
		if res != nil {
			res.Body.Close()
		}
		s.Close()

		common.Tracker = origTracker
	})

	Describe("POST /projects", func() {
		var (
			params  url.Values
			headers http.Header
		)

		BeforeEach(func() {
			params = url.Values{
				"name": {"foo-bar-express"},
			}
			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("POST", s.URL+"/projects", params, headers, nil)
			Expect(err).To(BeNil())
		}

		Context("when the project name is empty", func() {
			BeforeEach(func() {
				params.Del("name")
				doRequest()
			})

			It("returns 422 unprocessable entity", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(422))
				Expect(b.String()).To(MatchJSON(`{
					"error": "invalid_params",
					"errors": {
						"name": "is required"
					}
				}`))
			})
		})

		Context("when the project name is invalid", func() {
			BeforeEach(func() {
				params.Set("name", "foo-bar-")
				doRequest()
			})

			It("returns 422 unprocessable entity", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(422))
				Expect(b.String()).To(MatchJSON(`{
					"error": "invalid_params",
					"errors": {
						"name": "is invalid"
					}
				}`))
			})
		})

		Context("when the project name is taken", func() {
			BeforeEach(func() {
				proj2 := &project.Project{
					Name:   "foo-bar-express",
					UserID: u.ID,
				}

				err := db.Create(proj2).Error
				Expect(err).To(BeNil())

				doRequest()
			})

			It("returns 422 unprocessable entity", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(422))
				Expect(b.String()).To(MatchJSON(`{
					"error": "invalid_params",
					"errors": {
						"name": "is taken"
					}
				}`))
			})
		})

		Context("when the project name is blacklisted", func() {
			BeforeEach(func() {
				factories.BlacklistedName(db, "foo-bar-express")
				doRequest()
			})

			It("returns 422 unprocessable entity", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(422))
				Expect(b.String()).To(MatchJSON(`{
					"error": "invalid_params",
					"errors": {
						"name": "is taken"
					}
				}`))
			})

			It("tracks a 'Used Blacklisted Project Name' event", func() {
				trackCall := fakeTracker.TrackCalls.NthCall(1)
				Expect(trackCall).NotTo(BeNil())
				Expect(trackCall.Arguments[0]).To(Equal(fmt.Sprintf("%d", u.ID)))
				Expect(trackCall.Arguments[1]).To(Equal("Used Blacklisted Project Name"))
				Expect(trackCall.Arguments[2]).To(Equal(""))

				t := trackCall.Arguments[3]
				props, ok := t.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(props["projectName"]).To(Equal("foo-bar-express"))

				c := trackCall.Arguments[4]
				context, ok := c.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(context["ip"]).ToNot(BeNil())
				Expect(context["user_agent"]).ToNot(BeNil())

				Expect(trackCall.ReturnValues[0]).To(BeNil())
			})
		})

		Context("when the project name contains uppercase characters", func() {
			BeforeEach(func() {
				params.Set("name", "Foo-Bar-Express")
				doRequest()
			})

			It("converts those characters to lowercase", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				proj := &project.Project{}
				err = db.Last(proj).Error
				Expect(err).To(BeNil())

				createdAtJSON, err := proj.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusCreated))
				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project": {
						"name": "foo-bar-express",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": %s
					}
				}`, createdAtJSON)))
			})
		})

		Context("when the user has more than `MaxNumOfProject` projects", func() {
			var origMaxProjectPerUser int

			BeforeEach(func() {
				factories.Project(db, u)
				origMaxProjectPerUser = project.MaxProjectPerUser
				project.MaxProjectPerUser = 1

				params.Set("name", "foo-bar-express")
				doRequest()
			})

			AfterEach(func() {
				project.MaxProjectPerUser = origMaxProjectPerUser
			})

			It("returns 403 invalid request", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusForbidden))
				Expect(b.String()).To(MatchJSON(`{
					"error": "invalid_request",
					"error_description": "maximum number of projects reached"
				}`))
			})
		})

		Context("when a valid project name is given", func() {
			var proj *project.Project

			BeforeEach(func() {
				doRequest()
				proj = &project.Project{}
				err := db.Last(proj).Error
				Expect(err).To(BeNil())
			})

			It("returns 201 created", func() {
				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				proj := &project.Project{}
				err = db.Last(proj).Error
				Expect(err).To(BeNil())

				createdAtJSON, err := proj.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusCreated))
				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project": {
						"name": "foo-bar-express",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": %s
					}
				}`, createdAtJSON)))
			})

			It("creates a project record in the DB", func() {
				Expect(proj.Name).To(Equal("foo-bar-express"))
			})

			It("tracks a 'Created Project' event", func() {
				trackCall := fakeTracker.TrackCalls.NthCall(1)
				Expect(trackCall).NotTo(BeNil())
				Expect(trackCall.Arguments[0]).To(Equal(fmt.Sprintf("%d", u.ID)))
				Expect(trackCall.Arguments[1]).To(Equal("Created Project"))
				Expect(trackCall.Arguments[2]).To(Equal(""))

				t := trackCall.Arguments[3]
				props, ok := t.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(props["projectName"]).To(Equal("foo-bar-express"))

				c := trackCall.Arguments[4]
				context, ok := c.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(context["ip"]).ToNot(BeNil())
				Expect(context["user_agent"]).ToNot(BeNil())

				Expect(trackCall.ReturnValues[0]).To(BeNil())
			})
		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("GET /projects/:projectName", func() {
		var (
			proj *project.Project

			headers http.Header
		)

		BeforeEach(func() {
			proj = factories.Project(db, u)
			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("GET", s.URL+"/projects/"+proj.Name, nil, headers, nil)
			Expect(err).To(BeNil())
		}

		It("returns 200 OK and project json", func() {
			doRequest()

			b := &bytes.Buffer{}
			_, err := b.ReadFrom(res.Body)
			Expect(err).To(BeNil())

			proj := &project.Project{}
			err = db.Last(proj).Error
			Expect(err).To(BeNil())

			createdAtJSON, err := proj.CreatedAt.MarshalJSON()
			Expect(err).To(BeNil())

			Expect(res.StatusCode).To(Equal(http.StatusOK))
			Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
				"project": {
					"name": "%s",
					"default_domain_enabled": true,
					"force_https": false,
					"skip_build": false,
					"created_at": %s
				}
			}`, proj.Name, createdAtJSON)))
		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItRequiresProjectCollab(func() (*gorm.DB, *user.User, *project.Project) {
			return db, u, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("GET /projects", func() {
		var (
			headers http.Header

			anotherU *user.User

			proj  *project.Project
			proj2 *project.Project
			proj3 *project.Project
		)

		BeforeEach(func() {
			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}

			anotherU = factories.User(db)

			proj = factories.Project(db, u, "site-1")
			proj2 = factories.Project(db, anotherU, "site-2")
			proj3 = factories.Project(db, u, "site-3")
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("GET", s.URL+"/projects", nil, headers, nil)
			Expect(err).To(BeNil())
		}

		It("returns current user's projects ordered by name", func() {
			doRequest()

			b := &bytes.Buffer{}
			_, err := b.ReadFrom(res.Body)
			Expect(err).To(BeNil())

			proj := &project.Project{}
			err = db.Where("name = 'site-1'").First(proj).Error
			Expect(err).To(BeNil())

			createdAtJSON, err := proj.CreatedAt.MarshalJSON()
			Expect(err).To(BeNil())

			proj3 := &project.Project{}
			err = db.Where("name = 'site-3'").First(proj3).Error
			Expect(err).To(BeNil())

			createdAt3JSON, err := proj3.CreatedAt.MarshalJSON()
			Expect(err).To(BeNil())

			Expect(res.StatusCode).To(Equal(http.StatusOK))
			Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
				"projects": [
					{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": %s
					},
					{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": %s
					}
				],
				"shared_projects": []
			}`, proj.Name, createdAtJSON, proj3.Name, createdAt3JSON)))
		})

		Context("when user is a collaborator of other users' projects", func() {
			var (
				yetAnotherU *user.User
				proj4       *project.Project
				proj5       *project.Project
				proj6       *project.Project
			)

			BeforeEach(func() {
				yetAnotherU = factories.User(db)

				proj4 = factories.Project(db, anotherU, "site-4")
				proj5 = factories.Project(db, yetAnotherU, "site-5")
				proj6 = factories.Project(db, yetAnotherU, "site-6")

				err := proj4.AddCollaborator(db, u)
				Expect(err).To(BeNil())
				err = proj5.AddCollaborator(db, u)
				Expect(err).To(BeNil())
			})

			It("returns the shared projects ordered by name", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				proj := &project.Project{}
				err = db.Where("name = 'site-1'").First(proj).Error
				Expect(err).To(BeNil())

				createdAtJSON, err := proj.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				proj3 := &project.Project{}
				err = db.Where("name = 'site-3'").First(proj3).Error
				Expect(err).To(BeNil())

				createdAt3JSON, err := proj3.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				proj4 := &project.Project{}
				err = db.Where("name = 'site-4'").First(proj4).Error
				Expect(err).To(BeNil())

				createdAt4JSON, err := proj4.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				proj5 := &project.Project{}
				err = db.Where("name = 'site-5'").First(proj5).Error
				Expect(err).To(BeNil())

				createdAt5JSON, err := proj5.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))
				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"projects": [
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s
						},
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s
						}
					],
					"shared_projects": [
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s
						},
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s
						}
					]
				}`, proj.Name, createdAtJSON, proj3.Name, createdAt3JSON, proj4.Name, createdAt4JSON, proj5.Name, createdAt5JSON)))
			})
		})

		Context("when some projects have deployed and have collaborators", func() {
			var (
				depl  *deployment.Deployment
				depl4 *deployment.Deployment
				proj4 *project.Project
			)

			BeforeEach(func() {
				u2 := factories.User(db)

				proj4 = factories.Project(db, u2, "site-4")
				err := proj4.AddCollaborator(db, u)
				Expect(err).To(BeNil())

				depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
				factories.Deployment(db, proj3, u, deployment.StatePendingDeploy)
				depl4 = factories.Deployment(db, proj4, u2, deployment.StateDeployed)
			})

			It("responses with deployed at", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				// project 1
				Expect(db.First(proj, proj.ID).Error).To(BeNil())
				createdAtJSON, err := proj.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(db.First(depl, depl.ID).Error).To(BeNil())
				deployedAtJSON, err := depl.DeployedAt.MarshalJSON()
				Expect(err).To(BeNil())

				// project 3
				Expect(db.First(proj3, proj3.ID).Error).To(BeNil())
				createdAt3JSON, err := proj3.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				// project 4
				Expect(db.First(proj4, proj4.ID).Error).To(BeNil())
				createdAt4JSON, err := proj4.CreatedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(db.First(depl4, depl4.ID).Error).To(BeNil())
				deployedAt4JSON, err := depl4.DeployedAt.MarshalJSON()
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))
				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"projects": [
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s,
							"deployed_at": %s
						},
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s
						}
					],
					"shared_projects": [
						{
							"name": "%s",
							"default_domain_enabled": true,
							"force_https": false,
							"skip_build": false,
							"created_at": %s,
							"deployed_at": %s
						}
					]
				}`, proj.Name, createdAtJSON, deployedAtJSON,
					proj3.Name, createdAt3JSON,
					proj4.Name, createdAt4JSON, deployedAt4JSON,
				)))
			})
		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("PUT /projects/:name", func() {
		var (
			fakeS3                *fake.S3
			origS3                filetransfer.FileTransfer
			mq                    *amqp.Connection
			invalidationQueueName string

			proj *project.Project

			params  url.Values
			headers http.Header
		)

		BeforeEach(func() {
			origS3 = s3client.S3
			fakeS3 = &fake.S3{}
			s3client.S3 = fakeS3

			mq, err = mqconn.MQ()
			Expect(err).To(BeNil())

			testhelper.DeleteQueue(mq, queues.All...)
			testhelper.DeleteExchange(mq, exchanges.All...)

			invalidationQueueName = testhelper.StartQueueWithExchange(mq, exchanges.Edges, exchanges.RouteV1Invalidation)

			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}

			proj = factories.Project(db, u)
		})

		AfterEach(func() {
			s3client.S3 = origS3
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("PUT", s.URL+"/projects/"+proj.Name, params, headers, nil)
			Expect(err).To(BeNil())
		}

		Context("when default domain is newly disabled (i.e. it was enabled)", func() {
			BeforeEach(func() {
				Expect(proj.DefaultDomainEnabled).To(Equal(true))
				params = url.Values{
					"default_domain_enabled": {"false"},
				}
			})

			It("returns 200 OK", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())
				Expect(proj.DefaultDomainEnabled).To(Equal(false))

				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project":{
						"name": "%s",
						"default_domain_enabled": false,
						"force_https": false,
						"skip_build": false,
						"created_at": "%s"
					}
				}`, proj.Name, proj.CreatedAt.Format(time.RFC3339Nano))))
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("deletes the meta.json for the default domain from S3", func() {
					doRequest()

					Expect(fakeS3.DeleteCalls.Count()).To(Equal(1))

					deleteCall := fakeS3.DeleteCalls.NthCall(1)
					Expect(deleteCall).NotTo(BeNil())
					Expect(deleteCall.Arguments[0]).To(Equal(s3client.BucketRegion))
					Expect(deleteCall.Arguments[1]).To(Equal(s3client.BucketName))
					Expect(deleteCall.Arguments[2]).To(Equal("/domains/" + proj.Name + "." + shared.DefaultDomain + "/meta.json"))
					Expect(deleteCall.ReturnValues[0]).To(BeNil())
				})

				It("publishes invalidation message for the default domain", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, invalidationQueueName)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"domains": ["%s"]
					}`, proj.Name+"."+shared.DefaultDomain)))
				})
			})

			Context("when there is no active deployment", func() {
				It("does not delete the meta.json for the default domain from S3", func() {
					doRequest()

					Expect(fakeS3.DeleteCalls.Count()).To(Equal(0))
				})

				It("does not enqueue any job", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).To(BeNil())
				})
			})
		})

		Context("when default domain is newly enabled (i.e. it was disabled)", func() {
			BeforeEach(func() {
				proj.DefaultDomainEnabled = false
				Expect(db.Save(proj).Error).To(BeNil())
				params = url.Values{
					"default_domain_enabled": {"true"},
				}
			})

			It("returns 200 OK", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())
				Expect(proj.DefaultDomainEnabled).To(Equal(true))

				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project":{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": "%s"
					}
				}`, proj.Name, proj.CreatedAt.Format(time.RFC3339Nano))))
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("enqueues a deploy job to upload meta.json", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"deployment_id": %d,
						"skip_webroot_upload": true,
						"skip_invalidation": true,
						"use_raw_bundle": false
					}`, *proj.ActiveDeploymentID)))
				})
			})

			Context("when there is no active deployment", func() {
				It("does not enqueue any job", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).To(BeNil())
				})
			})
		})

		Context("when force_https is newly enabled (i.e. it was disabled)", func() {
			BeforeEach(func() {
				proj.ForceHTTPS = false
				Expect(db.Save(proj).Error).To(BeNil())
				params = url.Values{
					"force_https": {"true"},
				}
			})

			It("returns 200 OK", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())
				Expect(proj.DefaultDomainEnabled).To(Equal(true))

				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project":{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": true,
						"skip_build": false,
						"created_at": "%s"
					}
				}`, proj.Name, proj.CreatedAt.Format(time.RFC3339Nano))))
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("enqueues a deploy job to update meta.json", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"deployment_id": %d,
						"skip_webroot_upload": true,
						"skip_invalidation": false,
						"use_raw_bundle": false
					}`, *proj.ActiveDeploymentID)))
				})
			})
		})

		Context("when force_https is newly disabled (i.e. it was enabled)", func() {
			BeforeEach(func() {
				proj.ForceHTTPS = true
				Expect(db.Save(proj).Error).To(BeNil())
				params = url.Values{
					"force_https": {"false"},
				}
			})

			It("returns 200 OK", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())
				Expect(proj.DefaultDomainEnabled).To(Equal(true))

				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project":{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": false,
						"created_at": "%s"
					}
				}`, proj.Name, proj.CreatedAt.Format(time.RFC3339Nano))))
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("enqueues a deploy job to update meta.json", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"deployment_id": %d,
						"skip_webroot_upload": true,
						"skip_invalidation": false,
						"use_raw_bundle": false
					}`, *proj.ActiveDeploymentID)))
				})
			})
		})

		Context("when skip_build set to true", func() {
			BeforeEach(func() {
				proj.SkipBuild = false
				Expect(db.Save(proj).Error).To(BeNil())
				params = url.Values{
					"skip_build": {"true"},
				}
			})

			It("returns 200 OK", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())
				Expect(proj.DefaultDomainEnabled).To(Equal(true))

				Expect(b.String()).To(MatchJSON(fmt.Sprintf(`{
					"project":{
						"name": "%s",
						"default_domain_enabled": true,
						"force_https": false,
						"skip_build": true,
						"created_at": "%s"
					}
				}`, proj.Name, proj.CreatedAt.Format(time.RFC3339Nano))))
			})

		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItRequiresProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItLocksProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("DELETE /projects/:name", func() {
		var (
			fakeS3                *fake.S3
			origS3                filetransfer.FileTransfer
			mq                    *amqp.Connection
			invalidationQueueName string

			proj *project.Project
			dm1  *domain.Domain
			dm2  *domain.Domain

			headers http.Header
		)

		BeforeEach(func() {
			origS3 = s3client.S3
			fakeS3 = &fake.S3{}
			s3client.S3 = fakeS3

			mq, err = mqconn.MQ()
			Expect(err).To(BeNil())

			testhelper.DeleteQueue(mq, queues.All...)
			testhelper.DeleteExchange(mq, exchanges.All...)

			invalidationQueueName = testhelper.StartQueueWithExchange(mq, exchanges.Edges, exchanges.RouteV1Invalidation)

			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}

			proj = factories.Project(db, u)
			dm1 = factories.Domain(db, proj)
			dm2 = factories.Domain(db, proj)

			ct1 := &cert.Cert{
				DomainID:        dm1.ID,
				CertificatePath: "old/path",
				PrivateKeyPath:  "old/path",
			}
			Expect(db.Create(ct1).Error).To(BeNil())

			ct2 := &cert.Cert{
				DomainID:        dm2.ID,
				CertificatePath: "old/path",
				PrivateKeyPath:  "old/path",
			}
			Expect(db.Create(ct2).Error).To(BeNil())
		})

		AfterEach(func() {
			s3client.S3 = origS3
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("DELETE", s.URL+"/projects/"+proj.Name, nil, headers, nil)
			Expect(err).To(BeNil())
		}

		It("returns 200 with OK", func() {
			doRequest()
			b := &bytes.Buffer{}
			_, err := b.ReadFrom(res.Body)
			Expect(err).To(BeNil())

			Expect(res.StatusCode).To(Equal(http.StatusOK))
			Expect(b.String()).To(MatchJSON(`{
				"deleted": true
			}`))
		})

		It("deletes associated domains and certs", func() {
			doRequest()

			var count int
			Expect(db.Model(domain.Domain{}).Where("project_id = ?", proj.ID).Count(&count).Error).To(BeNil())
			Expect(count).To(Equal(0))

			Expect(db.Model(cert.Cert{}).Where("domain_id IN (?,?)", dm1.ID, dm2.ID).Count(&count).Error).To(BeNil())
			Expect(count).To(Equal(0))
		})

		It("deletes meta.json and ssl certs for the associated domains from s3", func() {
			doRequest()

			Expect(fakeS3.DeleteCalls.Count()).To(Equal(1))

			deleteCall := fakeS3.DeleteCalls.NthCall(1)
			Expect(deleteCall).NotTo(BeNil())
			Expect(deleteCall.Arguments[0]).To(Equal(s3client.BucketRegion))
			Expect(deleteCall.Arguments[1]).To(Equal(s3client.BucketName))
			Expect(deleteCall.ReturnValues[0]).To(BeNil())

			filesToDelete := []string{
				"domains/" + proj.DefaultDomainName() + "/meta.json",
				"domains/" + dm1.Name + "/meta.json",
				"certs/" + dm1.Name + "/ssl.crt",
				"certs/" + dm1.Name + "/ssl.key",
				"domains/" + dm2.Name + "/meta.json",
				"certs/" + dm2.Name + "/ssl.crt",
				"certs/" + dm2.Name + "/ssl.key",
			}

			for i, path := range filesToDelete {
				Expect(deleteCall.Arguments[2+i]).To(Equal(path))
			}
		})

		It("deletes the given project", func() {
			doRequest()
			Expect(db.First(&project.Project{}, proj.ID).Error).To(Equal(gorm.RecordNotFound))
		})

		It("publishes invalidation message for the associated domains", func() {
			doRequest()

			d := testhelper.ConsumeQueue(mq, invalidationQueueName)
			Expect(d).NotTo(BeNil())
			Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
				"domains": ["%s", "%s", "%s"]
			}`, proj.Name+"."+shared.DefaultDomain, dm1.Name, dm2.Name)))
		})

		It("tracks a 'Deleted Project' event", func() {
			doRequest()

			trackCall := fakeTracker.TrackCalls.NthCall(1)
			Expect(trackCall).NotTo(BeNil())
			Expect(trackCall.Arguments[0]).To(Equal(fmt.Sprintf("%d", u.ID)))
			Expect(trackCall.Arguments[1]).To(Equal("Deleted Project"))
			Expect(trackCall.Arguments[2]).To(Equal(""))

			t := trackCall.Arguments[3]
			props, ok := t.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(props["projectName"]).To(Equal(proj.Name))

			c := trackCall.Arguments[4]
			context, ok := c.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(context["ip"]).ToNot(BeNil())
			Expect(context["user_agent"]).ToNot(BeNil())

			Expect(trackCall.ReturnValues[0]).To(BeNil())
		})

		Context("when there are associated raw bundles", func() {
			var (
				bun1 *rawbundle.RawBundle
				bun2 *rawbundle.RawBundle
			)

			BeforeEach(func() {
				bun1 = factories.RawBundle(db, proj)
				bun2 = factories.RawBundle(db, proj)
			})

			It("deletes associated raw bundles from DB and S3", func() {
				doRequest()

				Expect(db.First(bun1, bun1.ID).Error).To(Equal(gorm.RecordNotFound))
				Expect(db.First(bun2, bun2.ID).Error).To(Equal(gorm.RecordNotFound))

				Expect(fakeS3.DeleteCalls.Count()).To(Equal(1))

				deleteCall := fakeS3.DeleteCalls.NthCall(1)
				Expect(deleteCall).NotTo(BeNil())
				Expect(deleteCall.Arguments[0]).To(Equal(s3client.BucketRegion))
				Expect(deleteCall.Arguments[1]).To(Equal(s3client.BucketName))
				Expect(deleteCall.ReturnValues[0]).To(BeNil())

				filesToDelete := []string{
					"domains/" + proj.DefaultDomainName() + "/meta.json",
					"domains/" + dm1.Name + "/meta.json",
					"certs/" + dm1.Name + "/ssl.crt",
					"certs/" + dm1.Name + "/ssl.key",
					"domains/" + dm2.Name + "/meta.json",
					"certs/" + dm2.Name + "/ssl.crt",
					"certs/" + dm2.Name + "/ssl.key",

					bun1.UploadedPath,
					bun2.UploadedPath,
				}

				for i, path := range filesToDelete {
					Expect(deleteCall.Arguments[2+i]).To(Equal(path))
				}
			})
		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItRequiresProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItLocksProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("POST /projects/:name/auth", func() {
		var (
			mq *amqp.Connection

			proj *project.Project

			params  url.Values
			headers http.Header
		)

		BeforeEach(func() {
			mq, err = mqconn.MQ()
			Expect(err).To(BeNil())

			testhelper.DeleteQueue(mq, queues.All...)

			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}

			proj = factories.Project(db, u)

			params = url.Values{
				"basic_auth_username": {"user"},
				"basic_auth_password": {"pass"},
			}
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("POST", s.URL+"/projects/"+proj.Name+"/auth", params, headers, nil)
			Expect(err).To(BeNil())
		}

		Context("`basic_auth_username` and `basic_auth_password` is provided", func() {
			It("returns 200 OK and updates the project", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(res.StatusCode).To(Equal(http.StatusOK))
				Expect(b.String()).To(MatchJSON(`{
					"protected": true
				}`))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())

				Expect(proj.BasicAuthUsername).NotTo(BeNil())
				Expect(*proj.BasicAuthUsername).To(Equal("user"))

				hasher := sha256.New()
				_, err = hasher.Write([]byte("user:pass"))
				Expect(err).To(BeNil())

				Expect(*proj.EncryptedBasicAuthPassword).To(Equal(hex.EncodeToString(hasher.Sum(nil))))
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("enqueues a deploy job to update meta.json", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"deployment_id": %d,
						"skip_webroot_upload": true,
						"skip_invalidation": false,
						"use_raw_bundle": false
					}`, *proj.ActiveDeploymentID)))
				})
			})
		})

		Context("when invalid params are provided", func() {
			DescribeTable("it returns 422 and does not update project",
				func(setUp func(), message string) {
					setUp()
					doRequest()

					b := &bytes.Buffer{}
					_, err := b.ReadFrom(res.Body)
					Expect(err).To(BeNil())

					Expect(res.StatusCode).To(Equal(422))
					Expect(b.String()).To(MatchJSON(message))

					err = db.First(proj, proj.ID).Error
					Expect(err).To(BeNil())

					Expect(proj.BasicAuthUsername).To(BeNil())
					Expect(proj.EncryptedBasicAuthPassword).To(BeNil())
				},

				Entry("require basic_auth_username", func() {
					params.Del("basic_auth_username")
				}, `{
						"error": "invalid_params",
						"errors": {
							"basic_auth_username": "is required"
						}
					}`),

				Entry("require basic_auth_password", func() {
					params.Del("basic_auth_password")
				}, `{
						"error": "invalid_params",
						"errors": {
							"basic_auth_password": "is required"
						}
					}`),
			)
		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItRequiresProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItLocksProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})

	Describe("DELETE /projects/:name/auth", func() {
		var (
			mq *amqp.Connection

			proj *project.Project

			headers http.Header
		)

		BeforeEach(func() {
			mq, err = mqconn.MQ()
			Expect(err).To(BeNil())

			testhelper.DeleteQueue(mq, queues.All...)

			headers = http.Header{
				"Authorization": {"Bearer " + t.Token},
			}

			proj = factories.Project(db, u)
			username := "user"
			password := "pass"
			proj.BasicAuthUsername = &username
			proj.BasicAuthPassword = password
			Expect(proj.EncryptBasicAuthPassword()).To(BeNil())
			Expect(db.Save(proj).Error).To(BeNil())
		})

		doRequest := func() {
			s = httptest.NewServer(server.New())
			res, err = testhelper.MakeRequest("DELETE", s.URL+"/projects/"+proj.Name+"/auth", nil, headers, nil)
			Expect(err).To(BeNil())
		}

		Context("`basic_auth_username` and `basic_auth_password` is provided", func() {
			It("returns 200 OK and updates the project", func() {
				doRequest()

				b := &bytes.Buffer{}
				_, err := b.ReadFrom(res.Body)
				Expect(err).To(BeNil())

				Expect(b.String()).To(MatchJSON(`{
					"unprotected": true
				}`))
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				err = db.First(proj, proj.ID).Error
				Expect(err).To(BeNil())

				Expect(proj.BasicAuthUsername).To(BeNil())
				Expect(proj.EncryptedBasicAuthPassword).To(BeNil())
			})

			Context("when there is an active deployment", func() {
				var depl *deployment.Deployment

				BeforeEach(func() {
					depl = factories.Deployment(db, proj, u, deployment.StateDeployed)
					err := db.Model(proj).Update("active_deployment_id", depl.ID).Error
					Expect(err).To(BeNil())
				})

				It("enqueues a deploy job to update meta.json", func() {
					doRequest()

					d := testhelper.ConsumeQueue(mq, queues.Deploy)
					Expect(d).NotTo(BeNil())
					Expect(d.Body).To(MatchJSON(fmt.Sprintf(`{
						"deployment_id": %d,
						"skip_webroot_upload": true,
						"skip_invalidation": false,
						"use_raw_bundle": false
					}`, *proj.ActiveDeploymentID)))
				})
			})

		})

		sharedexamples.ItRequiresAuthentication(func() (*gorm.DB, *user.User, *http.Header) {
			return db, u, &headers
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItRequiresProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)

		sharedexamples.ItLocksProject(func() (*gorm.DB, *project.Project) {
			return db, proj
		}, func() *http.Response {
			doRequest()
			return res
		}, nil)
	})
})
