+++
title = "Installing on Amazon EC2"
linktitle = "on Amazon EC2"
+++

To set up Sourcegraph on a new [Amazon EC2](https://aws.amazon.com/ec2/) instance, follow these steps.

* Open the [**Launch EC2 Instance Wizard**](https://us-west-2.console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard:) in the AWS Management Console.
* **Choose an AMI:** Ubuntu Server 14.04 AMI.
* **Choose an Instance Type:** Any instance with at least 2 GB of RAM (t2.small or better)
* **Configure Instance Details:** Expand **Edit instance details** then **Advanced details** and set the following **User data**:

```
{{% userdata SRC_AMAZON_EC2 %}}
```

* **Configure Security Group:** Allow external access to the following ports (or just choose All TCP).
  * Port 22 (for server administration via SSH)
  * Ports 80 and 443 (for the Web app)
* In 5 minutes, your Sourcegraph server should be available via HTTP at the EC2 instance's public IP or hostname.

## Questions?

* [cloud-init troubleshooting]({{< relref "troubleshooting/cloud-init.md" >}})

{{< ads_conversion >}}