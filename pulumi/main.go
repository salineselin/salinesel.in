package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/filestore"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// create a GCP disk that will be mapped as a pvc in kubernetes
		disk, err := compute.NewDisk(ctx, "salineselin", &compute.DiskArgs{
			Size:        pulumi.IntPtr(50),
			Zone:        pulumi.String("us-west3-c"),
			Description: pulumi.String("disk for hosting static site"),
		})
		if err != nil {
			return err
		}
		ctx.Export("disk id:", disk.ID())

		// create network file storage to put static site in
		_, err = filestore.NewInstance(ctx, "salineseldotin", &filestore.InstanceArgs{
			FileShares: &filestore.InstanceFileSharesArgs{
				CapacityGb: pulumi.Int(500),
				Name:       pulumi.String("staticsite"),
			},
			Location: pulumi.String("us-west3-c"),
			Networks: filestore.InstanceNetworkArray{
				&filestore.InstanceNetworkArgs{
					Modes: pulumi.StringArray{
						pulumi.String("MODE_IPV4"),
					},
					Network: pulumi.String("default"),
				},
			},
			Tier: pulumi.String("PREMIUM"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
