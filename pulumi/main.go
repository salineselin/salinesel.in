package main

import (
	"strings"

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

		// Create a GCP resource (Storage Bucket)
		bucket, err := storage.NewBucket(ctx, "salinesel-in", &storage.BucketArgs{
			Location:                 pulumi.String("US"),
			Name:                     pulumi.String("salinesel-in"),
			ForceDestroy:             pulumi.Bool(true),
			UniformBucketLevelAccess: pulumi.Bool(true),
			Website: &storage.BucketWebsiteArgs{
				MainPageSuffix: pulumi.String("index.html"),
				NotFoundPage:   pulumi.String("404.html"),
			},
			Cors: storage.BucketCorArray{
				&storage.BucketCorArgs{
					MaxAgeSeconds: pulumi.Int(3600),
					Methods: pulumi.StringArray{
						pulumi.String("GET"),
						pulumi.String("HEAD"),
						pulumi.String("PUT"),
						pulumi.String("POST"),
						pulumi.String("DELETE"),
					},
					Origins: pulumi.StringArray{
						pulumi.String("http://salinesel.in"),
						pulumi.String("https://salinesel.in"),
					},
					ResponseHeaders: pulumi.StringArray{
						pulumi.String("*"),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// add the 404 and index html pages
		pages := [...]string{
			"index.html",
			"404.html",
		}
		for _, page := range pages {
			_, err = storage.NewBucketObject(ctx, page, &storage.BucketObjectArgs{
				Bucket: bucket.Name,
				Source: pulumi.NewFileAsset(page),
			})
			if err != nil {
				return err
			}
		}

		// Create a serviceaccount
		saName := "salinesel-in-web"
		sa, err := serviceaccount.NewAccount(ctx, saName, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(saName),
			DisplayName: pulumi.String(saName),
			Description: pulumi.String("ServiceAccount used by Github Actions for seeding and serving content from the website bucket"),
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
				pulumi.String("serviceAccount:" + strings.TrimPrefix(project.Id, "projects/") + ".svc.id.goog[salinesel-in/salinesel-in]"),
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

		// give the serviceaccount permissions to create and remove DNS records in the salinesel.in domain

		// create a DNS record binding the bucket to a domain
		return nil
	})
}
