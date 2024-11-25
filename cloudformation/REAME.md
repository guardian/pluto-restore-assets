```aws cloudformation create-stack \
          --stack-name pluto-asset-restore-dev \
          --template-body file://template.yaml \
          --parameters \
              ParameterKey=AssetBuckets,ParameterValue="archivehunter-test-media\,archivehunter-test-
media-dave" \
              ParameterKey=ManifestBucket,ParameterValue=asset-restore-manifest-bucket \
              ParameterKey=RoleName,ParameterValue=pluto-asset-restore-role \
          --capabilities CAPABILITY_NAMED_IAM```

```aws iam attach-user-policy \
          --user-name pluto-asset-restore-dev \
          --policy-arn $(aws cloudformation describe-stacks \
          --stack-name pluto-asset-restore-dev \
          --query 'Stacks[0].Outputs[?OutputKey==`UserPolicyARN`].OutputValue' \
          --output text)
```
