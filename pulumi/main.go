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

		// define some common attributes
		base := "salinesel-in"
		serviceaccountname := base + "-web"
		projectId := strings.TrimPrefix(project.Id, "projects/") // remove projects/ prefix on project id

		// create a serviceaccount
		sa, err := serviceaccount.NewAccount(ctx, serviceaccountname, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(serviceaccountname),
			DisplayName: pulumi.String(serviceaccountname),
			Description: pulumi.String("ServiceAccount used by Github Actions for seeding and serving content from the website bucket"),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}

		// prefix the serviceaccount email with serviceAccount: for IAM
		saWithPrefix := sa.Email.ApplyT(func(Email string) string {
			return "serviceAccount:" + Email
		}).(pulumi.StringOutput)

		// make the serviceaccount a workload identity user
		name := fmt.Sprintf("give-%s-workload-identity-user", serviceaccountname)
		_, err = serviceaccount.NewIAMMember(ctx, name, &serviceaccount.IAMMemberArgs{
			ServiceAccountId: sa.Name,
			Role:             pulumi.String("roles/iam.workloadIdentityUser"),
			Member:           pulumi.Sprintf("serviceAccount:%s.svc.id.goog[salinesel-in/salinesel-in]", projectId),
		})
		if err != nil {
			return err
		}

		// give the serviceaccount permissions to the kubernetes cluster
		_, err = projects.NewIAMMember(ctx, name, &projects.IAMMemberArgs{
			Project: pulumi.String(projectId),
			Role:    pulumi.String("roles/container.developer"),
			Member:  saWithPrefix,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
