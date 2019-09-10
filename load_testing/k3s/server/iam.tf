resource "aws_iam_instance_profile" "k3s-server" {
  name_prefix = "load-testing-k3s-server"
  role        = aws_iam_role.k3s-server.name

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_iam_role" "k3s-server" {
  name_prefix = "load-testing-k3s-server"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF


  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_iam_role_policy" "k3s-server" {
  name_prefix = "load-testing-k3s-server"
  role        = aws_iam_role.k3s-server.id

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
              "ec2:AssociateAddress",
              "ec2:DescribeAddresses"
            ],
            "Sid": "",
            "Resource": [
                "*"
            ],
            "Effect": "Allow"
        }
    ]
}
EOF

}
