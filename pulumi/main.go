package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
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

		// get the existing salinesel.in zone
		// create a dns record mapping the ip address to salinesel.in
		// Create a serviceaccount
		name := site.serviceaccountName
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
