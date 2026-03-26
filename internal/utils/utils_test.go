package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils functions", func() {
	Context("Calling IsManagedByOperatorInstance", func() {
		When("operator's instance is not defined and the resource has no instance annotation", func() {

			It("should return true", func() {
				resourceAnnotations := map[string]string{}

				result := IsManagedByOperatorInstance(resourceAnnotations, "")

				Expect(result).To(BeTrue())
			})
		})

		When("operator's instance is defined and the resource has no instance annotation", func() {

			It("should return false", func() {
				resourceAnnotations := map[string]string{}

				result := IsManagedByOperatorInstance(resourceAnnotations, "foo")

				Expect(result).To(BeFalse())
			})
		})

		When("operator's instance is defined and the resource has another instance annotation", func() {

			It("should return false", func() {
				resourceAnnotations := map[string]string{
					OperatorInstanceAnnotationName: "bar",
				}

				result := IsManagedByOperatorInstance(resourceAnnotations, "foo")

				Expect(result).To(BeFalse())
			})
		})

		When("operator's instance is defined and the resource has the same another instance annotation", func() {

			It("should return false", func() {
				resourceAnnotations := map[string]string{
					OperatorInstanceAnnotationName: "foo",
				}

				result := IsManagedByOperatorInstance(resourceAnnotations, "foo")

				Expect(result).To(BeTrue())
			})
		})
	})

	Context("Calling GetLeaderElectionID", func() {
		When("operator's instance is not defined", func() {
			It("should return the default id", func() {
				result := GetLeaderElectionID("")
				Expect(result).To(Equal("default-37a8eec1.managed-postgres-operator.hoppscale.com"))
				Expect(len(result)).To(BeNumerically("<=", 63))
			})
		})

		When("operator's instance is defined and the name has a standard length", func() {
			It("should return the instance name with a unique id", func() {
				result := GetLeaderElectionID("foobar")
				Expect(result).To(Equal("foobar-c3ab8ff1.managed-postgres-operator.hoppscale.com"))
				Expect(len(result)).To(BeNumerically("<=", 63))
			})
		})

		When("operator's instance is defined and the name is too long", func() {
			It("should return the first few characters of the instance name and a unique ID", func() {
				result := GetLeaderElectionID("myverylonginstancename")
				Expect(result).To(Equal("myverylonginst-3b8d6ea3.managed-postgres-operator.hoppscale.com"))
				Expect(len(result)).To(BeNumerically("<=", 63))
			})
		})

		When("multiple instances begin with the same characters", func() {
			It("shouldn't cause a conflict", func() {
				result1 := GetLeaderElectionID("postgresinstance-dev")
				result2 := GetLeaderElectionID("postgresinstance-int")
				result3 := GetLeaderElectionID("postgresinstance-test")
				result4 := GetLeaderElectionID("postgresinstance-prod")

				Expect(result1).NotTo(BeElementOf([]string{result2, result3, result4}))
				Expect(result2).NotTo(BeElementOf([]string{result1, result3, result4}))
				Expect(result3).NotTo(BeElementOf([]string{result1, result2, result4}))
				Expect(result4).NotTo(BeElementOf([]string{result1, result2, result3}))
			})
		})
	})
})
