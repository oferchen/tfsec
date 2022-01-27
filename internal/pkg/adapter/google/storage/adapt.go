package storage

import (
	"github.com/aquasecurity/defsec/provider/google/storage"
	"github.com/aquasecurity/tfsec/internal/pkg/block"
)

func Adapt(modules block.Modules) storage.Storage {
	return storage.Storage{
		Buckets: (&adapter{modules: modules}).adaptBuckets(),
	}
}

type adapter struct {
	modules    block.Modules
	bindings   []parentedBinding
	members    []parentedMember
	bindingMap block.ResourceIDResolutions
	memberMap  block.ResourceIDResolutions
}

func (a *adapter) adaptBuckets() []storage.Bucket {

	a.bindingMap = a.modules.GetChildResourceIDMapByType("google_storage_bucket_iam_binding", "google_storage_bucket_iam_policy")
	a.memberMap = a.modules.GetChildResourceIDMapByType("google_storage_bucket_iam_member")

	a.adaptMembers()
	a.adaptBindings()

	var buckets []storage.Bucket
	for _, module := range a.modules {
		for _, resource := range module.GetResourcesByType("google_storage_bucket") {
			buckets = append(buckets, a.adaptBucketResource(resource))
		}
	}

	var orphanage storage.Bucket
	for _, orphanedBindingID := range a.bindingMap.Orphans() {
		for _, binding := range a.bindings {
			if binding.blockID == orphanedBindingID {
				orphanage.Bindings = append(orphanage.Bindings, binding.bindings...)
				break
			}
		}
	}
	for _, orphanedMemberID := range a.memberMap.Orphans() {
		for _, member := range a.members {
			if member.blockID == orphanedMemberID {
				orphanage.Members = append(orphanage.Members, member.member)
				break
			}
		}
	}
	if len(orphanage.Bindings) > 0 || len(orphanage.Members) > 0 {
		buckets = append(buckets, orphanage)
	}

	return buckets
}

func (a *adapter) adaptBucketResource(resourceBlock *block.Block) storage.Bucket {

	nameAttr := resourceBlock.GetAttribute("name")
	nameValue := nameAttr.AsStringValueOrDefault("", resourceBlock)

	locationAttr := resourceBlock.GetAttribute("location")
	locationValue := locationAttr.AsStringValueOrDefault("", resourceBlock)

	// See https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/storage_bucket#uniform_bucket_level_access
	ublaAttr := resourceBlock.GetAttribute("uniform_bucket_level_access")
	ublaValue := ublaAttr.AsBoolValueOrDefault(false, resourceBlock)

	bucket := storage.Bucket{
		Metadata:                       resourceBlock.Metadata(),
		Name:                           nameValue,
		Location:                       locationValue,
		EnableUniformBucketLevelAccess: ublaValue,
	}

	var name string
	if nameAttr.IsString() {
		name = nameAttr.Value().AsString()
	}

	for _, member := range a.members {
		if member.bucketBlockID == resourceBlock.ID() {
			bucket.Members = append(bucket.Members, member.member)
			a.memberMap.Resolve(member.blockID)
			continue
		}
		if name != "" && name == member.bucketID {
			bucket.Members = append(bucket.Members, member.member)
			a.memberMap.Resolve(member.blockID)
		}
	}
	for _, binding := range a.bindings {
		if binding.bucketBlockID == resourceBlock.ID() {
			bucket.Bindings = append(bucket.Bindings, binding.bindings...)
			a.bindingMap.Resolve(binding.blockID)
			continue
		}
		if name != "" && name == binding.bucketID {
			bucket.Bindings = append(bucket.Bindings, binding.bindings...)
			a.bindingMap.Resolve(binding.blockID)
		}
	}

	return bucket
}