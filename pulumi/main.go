package main

import (
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/projects"
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
		// remove projects/ prefix on project id
		projectId := strings.TrimPrefix(project.Id, "projects/")

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

		// allow people online to view the contents of the bucket
		_, err = storage.NewBucketIAMBinding(ctx, "salinesel-in-bucket-iambinding", &storage.BucketIAMBindingArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.objectViewer"),
			Members: pulumi.StringArray{
				pulumi.String("allUsers"),
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
				Bucket:      bucket.Name,
				ContentType: pulumi.String("text/html"),
				Source:      pulumi.NewFileAsset(page),
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
				pulumi.String("serviceAccount:" + projectId + ".svc.id.goog[salinesel-in/salinesel-in]"),
			},
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to read and write to the website bucket for CI
		_, err = storage.NewBucketIAMMember(ctx, "give-sa-bucket-permissions", &storage.BucketIAMMemberArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.admin"),
			Member: saWithPrefix,
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to create and remove DNS records in the salinesel.in domain
		_, err = projects.NewIAMMember(ctx, "give-sa-dns-rw", &projects.IAMMemberArgs{
			Project: pulumi.String(projectId),
			Role:    pulumi.String("roles/dns.admin"),
			Member:  saWithPrefix,
			Condition: &projects.IAMMemberConditionArgs{
				Description: pulumi.String("only for the salinesel.in zone"),
				Title:       pulumi.String("only-salinesel.in"),
				Expression:  pulumi.String("resource.name == \"salinesel-in\""),
			},
		})
		if err != nil {
			return err
		}

		return nil
	})
}
