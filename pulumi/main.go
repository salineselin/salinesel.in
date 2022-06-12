package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// add google beta provider
		projectName := config.Get(ctx, "gcp:project")
		google_beta, err := gcp.NewProvider(ctx, "google-beta", &gcp.ProviderArgs{
			Project: pulumi.String(projectName),
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

		// log its name
		ctx.Export("serviceaccount", sa.Email)

		// stub in the serviceaccount string in front of the serviceaccounts email
		saWithPrefix := sa.Email.ApplyT(func(Email string) string {
			return "serviceAccount:" + Email
		}).(pulumi.StringOutput)

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
