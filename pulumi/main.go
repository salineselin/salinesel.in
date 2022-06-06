package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// create a GCP disk that will be mapped as a pvc in kubernetes
		_, err := compute.NewDisk(ctx, "salinesel.in website disk", &compute.DiskArgs{
			Size:        pulumi.IntPtr(50),
			Zone:        pulumi.String("us-west3-c"),
			Description: pulumi.String("disk for hosting static site"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
