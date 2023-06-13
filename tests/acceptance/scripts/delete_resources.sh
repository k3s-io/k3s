#!/bin/bash

#Get resource name from tfvarslocal && change name to make more sense in this context
RESOURCE_NAME=$(grep resource_name <modules/k3scluster/config/local.tfvars | cut -d= -f2 | tr -d ' "')
NAME_PREFIX="$RESOURCE_NAME"

#Terminate the instances
echo "Terminating resources for $NAME_PREFIX if still up and running"
# shellcheck disable=SC2046
aws ec2 terminate-instances --instance-ids $(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=${NAME_PREFIX}*" \
  "Name=instance-state-name,Values=running" --query \
  'Reservations[].Instances[].InstanceId' --output text) > /dev/null 2>&1


#Search for DB instances and delete them
INSTANCES=$(aws rds describe-db-instances --query "DBInstances[?starts_with(DBInstanceIdentifier,
'${NAME_PREFIX}')].DBInstanceIdentifier" --output text 2> /dev/null)
for instance in $INSTANCES; do
  aws rds delete-db-instance --db-instance-identifier "$instance" --skip-final-snapshot > /dev/null 2>&1
done


#Search for DB clusters and delete them
CLUSTERS=$(aws rds describe-db-clusters --query "DBClusters[?starts_with(DBClusterIdentifier,
 '${NAME_PREFIX}')].DBClusterIdentifier" --output text 2> /dev/null)
for cluster in $CLUSTERS; do
  aws rds delete-db-cluster --db-cluster-identifier "$cluster" --skip-final-snapshot > /dev/null 2>&1
  aws rds wait db-cluster-deleted --db-cluster-identifier "$cluster"
done


#Search for DB snapshots and delete them
SNAPSHOTS=$(aws rds describe-db-snapshots --query "DBSnapshots[?starts_with(DBSnapshotIdentifier,
 '${NAME_PREFIX}')].DBSnapshotIdentifier" --output text 2> /dev/null)
for snapshot in $SNAPSHOTS; do
  aws rds delete-db-snapshot --db-snapshot-identifier "$snapshot" > /dev/null 2>&1
done


#Get the list of load balancer ARNs
LB_ARN_LIST=$(aws elbv2 describe-load-balancers \
  --query "LoadBalancers[?starts_with(LoadBalancerName, '${NAME_PREFIX}') && Type=='network'].LoadBalancerArn" \
  --output text)


#Loop through the load balancer ARNs and delete the load balancers
for LB_ARN in $LB_ARN_LIST; do
  echo "Deleting load balancer $LB_ARN"
  aws elbv2 delete-load-balancer --load-balancer-arn "$LB_ARN"
done

#Get the list of target group ARNs
TG_ARN_LIST=$(aws elbv2 describe-target-groups \
  --query "TargetGroups[?starts_with(TargetGroupName, '${NAME_PREFIX}') && Protocol=='TCP'].TargetGroupArn" \
  --output text)


#Loop through the target group ARNs and delete the target groups
for TG_ARN in $TG_ARN_LIST; do
  echo "Deleting target group $TG_ARN"
  aws elbv2 delete-target-group --target-group-arn "$TG_ARN"
done


#Get the ID and recordName with lower case of the hosted zone that contains the Route 53 record sets
NAME_PREFIX_LOWER=$(echo "$NAME_PREFIX" | tr '[:upper:]' '[:lower:]')
R53_ZONE_ID=$(aws route53 list-hosted-zones-by-name --dns-name "${NAME_PREFIX}." \
  --query "HostedZones[0].Id" --output text)
R53_RECORD=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "${R53_ZONE_ID}" \
  --query "ResourceRecordSets[?starts_with(Name, '${NAME_PREFIX_LOWER}.') && Type == 'CNAME'].Name" \
  --output text)


#Get ResourceRecord Value
RECORD_VALUE=$(aws route53 list-resource-record-sets \
  --hosted-zone-id "${R53_ZONE_ID}" \
  --query "ResourceRecordSets[?starts_with(Name, '${NAME_PREFIX_LOWER}.') \
    && Type == 'CNAME'].ResourceRecords[0].Value" --output text)


#Delete Route53 record
if [[ "$R53_RECORD" == "${NAME_PREFIX_LOWER}."* ]]; then
  echo "Deleting Route53 record ${R53_RECORD}"
  CHANGE_STATUS=$(aws route53 change-resource-record-sets --hosted-zone-id "${R53_ZONE_ID}" \
    --change-batch '{"Changes": [
            {
                "Action": "DELETE",
                "ResourceRecordSet": {
                    "Name": "'"${R53_RECORD}"'",
                    "Type": "CNAME",
                    "TTL": 300,
                    "ResourceRecords": [
                        {
                            "Value": "'"${RECORD_VALUE}"'"
                        }
                    ]
                }
            }
        ]
    }')
  STATUS_ID=$(echo "$CHANGE_STATUS" | jq -r '.ChangeInfo.Id')
  #Get status from the change
  aws route53 wait resource-record-sets-changed --id "$STATUS_ID"
  echo "Successfully deleted Route53 record ${R53_RECORD}: status: ${STATUS_ID}"
else
  echo "No Route53 record found"
fi