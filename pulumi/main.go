package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
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

		// create a backend bucket associated with the storage bucket
		name = fmt.Sprintf("%s-backendbucket", site.bucketName)
		backendBucket, err := compute.NewBackendBucket(ctx, name, &compute.BackendBucketArgs{
			Name:       pulumi.String(name),
			BucketName: bucket.Name,
			EnableCdn:  pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		// create a health check
		name = fmt.Sprintf("%s-healthcheck", site.bucketName)
		healthCheck, err := compute.NewHttpHealthCheck(ctx, name, &compute.HttpHealthCheckArgs{
			RequestPath:      pulumi.String("/"),
			CheckIntervalSec: pulumi.Int(1),
			TimeoutSec:       pulumi.Int(1),
		})
		if err != nil {
			return err
		}

		// create a http backendservice
		name = fmt.Sprintf("%s-backendservice", site.bucketName)
		backendService, err := compute.NewBackendService(ctx, name, &compute.BackendServiceArgs{
			PortName:     pulumi.String("http"),
			Protocol:     pulumi.String("HTTP"),
			TimeoutSec:   pulumi.Int(10),
			HealthChecks: healthCheck.ID(),
		})
		if err != nil {
			return err
		}

		// create url map for backend bucket
		// https://www.pulumi.com/registry/packages/gcp/api-docs/compute/targethttpsproxy/
		// https://stackoverflow.com/questions/66161921/setting-up-load-balancer-frontend-with-on-gcp-with-pulumi
		name = fmt.Sprintf("%s-urlmap", site.bucketName)
		urlmap, err := compute.NewURLMap(ctx, name, &compute.URLMapArgs{
			DefaultService: backendBucket.ID(),
			HostRules: compute.URLMapHostRuleArray{
				&compute.URLMapHostRuleArgs{
					Hosts: pulumi.StringArray{
						pulumi.String(site.apexDomain),
					},
					PathMatcher: pulumi.String(site.bucketName),
				},
			},
			PathMatchers: compute.URLMapPathMatcherArray{
				&compute.URLMapPathMatcherArgs{
					Name:           pulumi.String(site.bucketName),
					DefaultService: backendBucket.ID(),
					PathRules: compute.URLMapPathMatcherPathRuleArray{
						&compute.URLMapPathMatcherPathRuleArgs{
							Service: backendService.ID(),
							Paths: pulumi.StringArray{
								pulumi.String("/*"),
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// create an SSL certificate to encrypt the frontend
		name = fmt.Sprintf("%s-ssl-cert", site.bucketName)
		cert, err := compute.NewManagedSslCertificate(ctx, name, &compute.ManagedSslCertificateArgs{
			Name: pulumi.String(name),
			Managed: compute.ManagedSslCertificateManagedArgs{
				Domains: pulumi.StringArray{
					pulumi.String(site.domain),
				},
			},
		})
		if err != nil {
			return err
		}

		// bind the https certificate to a load balancer
		name = fmt.Sprintf("%s-https-proxy", site.bucketName)
		_, err = compute.NewTargetHttpsProxy(ctx, name, &compute.TargetHttpsProxyArgs{
			UrlMap: urlmap.ID(),
			SslCertificates: pulumi.StringArray{
				cert.ID(),
			},
		})
		if err != nil {
			return err
		}

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
