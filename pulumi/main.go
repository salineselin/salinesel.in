package main

import (
	"fmt"
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

		// define some common attributes with an anonymous struct
		base := "salinesel-in"
		domain := "salinesel.in"
		site := struct {
			bucketName         string
			backendBucketName  string
			serviceaccountName string
			domain             string
			apexDomain         string
			TopLevelDomain     string
			projectId          string
		}{
			bucketName:         base,
			serviceaccountName: base + "-web",
			domain:             domain,
			apexDomain:         strings.Split(domain, ".")[0],
			TopLevelDomain:     strings.Split(domain, ".")[1],
			projectId:          strings.TrimPrefix(project.Id, "projects/"), // remove projects/ prefix on project id
		}

		// create storage bucket
		name := fmt.Sprintf("%s-bucket", site.bucketName)
		bucket, err := storage.NewBucket(ctx, name, &storage.BucketArgs{
			Location:                 pulumi.String("US"),
			Name:                     pulumi.String(name),
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
						pulumi.String("http://" + site.domain),
						pulumi.String("https://" + site.domain),
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
		name = fmt.Sprintf("%s-bucket-iambinding", site.bucketName)
		_, err = storage.NewBucketIAMBinding(ctx, name, &storage.BucketIAMBindingArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.objectViewer"),
			Members: pulumi.StringArray{
				pulumi.String("allUsers"),
			},
		})
		if err != nil {
			return err
		}

		// get the existing salinesel.in zone

		// create a dns record mapping the ip address to salinesel.in

		// Create a serviceaccount
		name = site.serviceaccountName
		sa, err := serviceaccount.NewAccount(ctx, name, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(name),
			DisplayName: pulumi.String(name),
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
		name = fmt.Sprintf("give-%s-workload-identity-user", site.serviceaccountName)
		_, err = serviceaccount.NewIAMMember(ctx, name, &serviceaccount.IAMMemberArgs{
			ServiceAccountId: sa.Name,
			Role:             pulumi.String("roles/iam.workloadIdentityUser"),
			Member:           pulumi.Sprintf("serviceAccount:%s.svc.id.goog[salinesel-in/salinesel-in]", site.projectId),
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to read and write to the website bucket for CI
		name = fmt.Sprintf("give-%s-bucket-rw", site.serviceaccountName)
		_, err = storage.NewBucketIAMMember(ctx, name, &storage.BucketIAMMemberArgs{
			Bucket: bucket.Name,
			Role:   pulumi.String("roles/storage.admin"),
			Member: saWithPrefix,
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to create and remove DNS records in the salinesel.in domain
		name = fmt.Sprintf("give-%s-dns-rw", site.serviceaccountName)
		_, err = projects.NewIAMMember(ctx, name, &projects.IAMMemberArgs{
			Project: pulumi.String(site.projectId),
			Role:    pulumi.String("roles/dns.admin"),
			Member:  saWithPrefix,
			Condition: &projects.IAMMemberConditionArgs{
				Description: pulumi.Sprintf("only for the %s zone", site.domain),
				Title:       pulumi.Sprintf("only-%s", site.bucketName),
				Expression:  pulumi.Sprintf("resource.name == \"%s\"", site.bucketName),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
