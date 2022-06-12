package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// get complete project metadata used by functions
		project, err := organizations.LookupProject(ctx, nil, nil)
		if err != nil {
			return err
		}

		// add google beta provider
		google_beta, err := gcp.NewProvider(ctx, "google-beta", &gcp.ProviderArgs{
			Project: pulumi.String(project.Id),
		})
		if err != nil {
			return err
		}

		// Create a GCP resource (Storage Bucket)
		bucket, err := storage.NewBucket(ctx, "salinesel-in", &storage.BucketArgs{
			Location: pulumi.String("US"),
		})
		if err != nil {
			return err
		}

		// Create a serviceaccount
		saName := "salinesel-in-web"
		sa, err := serviceaccount.NewAccount(ctx, saName, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(saName),
			DisplayName: pulumi.String(saName),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}

		// stub in the serviceaccount string in front of the serviceaccounts email
		saWithPrefix := sa.Email.ApplyT(func(Email string) string {
			return "serviceAccount:" + Email
		}).(pulumi.StringOutput)

		// make the serviceaccount a workload identity user
		_, err = serviceaccount.NewIAMBinding(ctx, "make-sa-workload-identity-user", &serviceaccount.IAMBindingArgs{
			ServiceAccountId: sa.Name,
			Role:             pulumi.String("roles/iam.workloadIdentityUser"),
			Members: pulumi.StringArray{
				pulumi.String("serviceAccount:" + project.Id + ".svc.id.goog[salinesel-in/salinesel-in]"),
			},
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to read the website bucket
		_, err = storage.NewBucketIAMMember(ctx, "give-sa-bucket-permissions", &storage.BucketIAMMemberArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.admin"),
			Member: saWithPrefix,
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to read the artifact registry
		_, err = artifactregistry.NewRepositoryIamMember(ctx, "give-sa-registry-read", &artifactregistry.RepositoryIamMemberArgs{
			Location:   pulumi.String("us-west3"),
			Repository: pulumi.String("salinesel-in"),
			Role:       pulumi.String("roles/artifactregistry.reader"),
			Member:     saWithPrefix,
		}, pulumi.Provider(google_beta))
		if err != nil {
			return err
		}

		return nil
	})
}
