//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *API) DeepCopyInto(out *API) {
	*out = *in
	in.PodSpec.DeepCopyInto(&out.PodSpec)
	if in.Request != nil {
		in, out := &in.Request, &out.Request
		*out = new(Request)
		(*in).DeepCopyInto(*out)
	}
	if in.Response != nil {
		in, out := &in.Response, &out.Response
		*out = new(Response)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new API.
func (in *API) DeepCopy() *API {
	if in == nil {
		return nil
	}
	out := new(API)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APISet) DeepCopyInto(out *APISet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APISet.
func (in *APISet) DeepCopy() *APISet {
	if in == nil {
		return nil
	}
	out := new(APISet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *APISet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APISetList) DeepCopyInto(out *APISetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]APISet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APISetList.
func (in *APISetList) DeepCopy() *APISetList {
	if in == nil {
		return nil
	}
	out := new(APISetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *APISetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APISetSpec) DeepCopyInto(out *APISetSpec) {
	*out = *in
	if in.APIs != nil {
		in, out := &in.APIs, &out.APIs
		*out = make([]API, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Kcgid != nil {
		in, out := &in.Kcgid, &out.Kcgid
		*out = new(Kcgid)
		(*in).DeepCopyInto(*out)
	}
	if in.HoistImages != nil {
		in, out := &in.HoistImages, &out.HoistImages
		*out = new(bool)
		**out = **in
	}
	if in.HistoryLimit != nil {
		in, out := &in.HistoryLimit, &out.HistoryLimit
		*out = new(HistoryLimit)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APISetSpec.
func (in *APISetSpec) DeepCopy() *APISetSpec {
	if in == nil {
		return nil
	}
	out := new(APISetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APISetStatus) DeepCopyInto(out *APISetStatus) {
	*out = *in
	if in.ServiceAccount != nil {
		in, out := &in.ServiceAccount, &out.ServiceAccount
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.RoleBinding != nil {
		in, out := &in.RoleBinding, &out.RoleBinding
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Deployment != nil {
		in, out := &in.Deployment, &out.Deployment
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Ingress != nil {
		in, out := &in.Ingress, &out.Ingress
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.ImagePullSecret != nil {
		in, out := &in.ImagePullSecret, &out.ImagePullSecret
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.ServiceMonitor != nil {
		in, out := &in.ServiceMonitor, &out.ServiceMonitor
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Deployed != nil {
		in, out := &in.Deployed, &out.Deployed
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APISetStatus.
func (in *APISetStatus) DeepCopy() *APISetStatus {
	if in == nil {
		return nil
	}
	out := new(APISetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HistoryLimit) DeepCopyInto(out *HistoryLimit) {
	*out = *in
	in.Succeeded.DeepCopyInto(&out.Succeeded)
	in.Failed.DeepCopyInto(&out.Failed)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HistoryLimit.
func (in *HistoryLimit) DeepCopy() *HistoryLimit {
	if in == nil {
		return nil
	}
	out := new(HistoryLimit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HistoryLimitSpec) DeepCopyInto(out *HistoryLimitSpec) {
	*out = *in
	if in.MaxCount != nil {
		in, out := &in.MaxCount, &out.MaxCount
		*out = new(int32)
		**out = **in
	}
	if in.MaxAge != nil {
		in, out := &in.MaxAge, &out.MaxAge
		*out = new(int32)
		**out = **in
	}
	if in.KeepPreviousVersions != nil {
		in, out := &in.KeepPreviousVersions, &out.KeepPreviousVersions
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HistoryLimitSpec.
func (in *HistoryLimitSpec) DeepCopy() *HistoryLimitSpec {
	if in == nil {
		return nil
	}
	out := new(HistoryLimitSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Kcgid) DeepCopyInto(out *Kcgid) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Kcgid.
func (in *Kcgid) DeepCopy() *Kcgid {
	if in == nil {
		return nil
	}
	out := new(Kcgid)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Request) DeepCopyInto(out *Request) {
	*out = *in
	if in.Schema != nil {
		in, out := &in.Schema, &out.Schema
		*out = new(Schema)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Request.
func (in *Request) DeepCopy() *Request {
	if in == nil {
		return nil
	}
	out := new(Request)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Response) DeepCopyInto(out *Response) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Response.
func (in *Response) DeepCopy() *Response {
	if in == nil {
		return nil
	}
	out := new(Response)
	in.DeepCopyInto(out)
	return out
}
