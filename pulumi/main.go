package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a GCP resource (Storage Bucket)
		bucket, err := storage.NewBucket(ctx, "salinesel-in", &storage.BucketArgs{
			Location: pulumi.String("US"),
		})
		if err != nil {
			return err
		}

		// Create a serviceaccount
		saName := "salinesel-in-bucket-read"
		sa, err := serviceaccount.NewAccount(ctx, saName, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(saName),
			DisplayName: pulumi.String(saName),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}

		// log its name
		ctx.Export("serviceaccount", sa.Email)

		// give the serviceaccount permissions to read the bucket
		_, err = storage.NewBucketIAMMember(ctx, "give-sa-bucket-permissions", &storage.BucketIAMMemberArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.objects.get"),
			Member: sa.Email.ApplyT(func(Email string) string {
				return "serviceAccount:" + Email
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
