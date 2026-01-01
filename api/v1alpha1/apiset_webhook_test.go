package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildAPISet(path, schema string) *APISet {
	return &APISet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: APISetSpec{
			Host: "example.local",
			APIs: []API{
				{
					Path: path,
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
	DescribeTable("when creating APISet",
		func(ctx SpecContext, path, schema, msg string) {
			err := k8sClient.Create(ctx, buildAPISet(path, schema))
			if msg == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(msg))
			}
		},
		Entry("accepts when schema and path are valid", "/valid", `{"type": "object"}`, ""),

		Entry("rejects when schema is not valid", "/valid", `{"type": "invalid"}`, "spec.apis[0].request.schema"),
		Entry("rejects when path is not valid", "/{invalid", `{"type": "object"}`, "spec.apis[0].path"),
	)
})
