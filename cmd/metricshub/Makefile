deploy-testnet:
	gcloud run deploy metricshub-testnet --source=${PWD}  --set-env-vars="GCP_PROJECT=textile-310716,BIGQUERY_DATASET=tableland_testnet,BIGQUERY_TABLE=system_metrics" --region="us-west1" --allow-unauthenticated

deploy-staging:
	gcloud run deploy metricshub-staging --source=${PWD}  --set-env-vars="GCP_PROJECT=textile-310716,BIGQUERY_DATASET=tableland_staging,BIGQUERY_TABLE=system_metrics" --region="us-west1" --allow-unauthenticated