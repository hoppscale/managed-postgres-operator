package postgresql

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

var _ = Describe("PostgreSQL Pool", func() {
	Context("Calling EnsurePGPoolExists", func() {
		When("the pool already exists", func() {
			It("should return no error", func() {
				mock, err := pgxmock.NewPool()
				if err != nil {
					Fail(err.Error())
				}
				pgpools := PGPools{
					Default: mock,
					Databases: map[string]PGPoolInterface{
						"test": mock,
					},
				}
				err = EnsurePGPoolExists(&pgpools, "test")

				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the pool doesn't exist and the default config is correct", func() {
			It("should return no error", func() {
				mock, err := pgxmock.NewPool()
				if err != nil {
					Fail(err.Error())
				}
				pgpools := PGPools{
					Default:   mock,
					Databases: map[string]PGPoolInterface{},
				}
				err = EnsurePGPoolExists(&pgpools, "test")

				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the pool doesn't exist and there is no default config", func() {
			It("should return an error", func() {
				pgpools := PGPools{
					Databases: map[string]PGPoolInterface{},
				}
				err := EnsurePGPoolExists(&pgpools, "test")

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
