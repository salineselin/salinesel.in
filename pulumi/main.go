package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
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

		// create a disk to use in the GCP cluster
		_, err = compute.NewDisk(ctx, base+"-disk", &compute.DiskArgs{
			Zone: pulumi.String("us-west3-c"),
			Size: pulumi.Int(10),
			Type: pulumi.String("pd-ssd"),
			Name: pulumi.String("salinesel-in-nfs-disk"),
		})
		if err != nil {
			return err
		}

		// create a serviceaccount
		sa, err := serviceaccount.NewAccount(ctx, serviceaccountname, &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(serviceaccountname),
			DisplayName: pulumi.String(serviceaccountname),
			Description: pulumi.String("ServiceAccount used by Github Actions for seeding and serving content from the website bucket"),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}

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

		return nil
	})
}
