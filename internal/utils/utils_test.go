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
})
