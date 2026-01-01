package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildAPISetWithSchema(schema string) *APISet {
	return &APISet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: APISetSpec{
			Host: "example.local",
			APIs: []API{
				{
					Path: "/test",
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "alpine:latest",
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
					Request: &Request{
						Schema: &Schema{
							RawJSON: schema,
						},
					},
				},
			},
		},
	}
}

var _ = Describe("validation webhook", func() {
	Context("when creating APISet", func() {
		It("accepts when schema is not valid", func(ctx SpecContext) {
			err := k8sClient.Create(ctx, buildAPISetWithSchema(`{"type": "object"}`))
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects when schema is not valid", func(ctx SpecContext) {
			err := k8sClient.Create(ctx, buildAPISetWithSchema(`{"type": "invalid"}`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.apis[0].request.schema"))
		})
	})
})
