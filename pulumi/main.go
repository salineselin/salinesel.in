package main

import (
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/filestore"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// create network file storage to put static site in
		_, err := filestore.NewInstance(ctx, "static-site-filestore", &filestore.InstanceArgs{
			FileShares: &filestore.InstanceFileSharesArgs{
				CapacityGb: pulumi.Int(1024),
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
			Tier: pulumi.String("STANDARD"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
