package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsServiceQuotasServiceQuota() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsServiceQuotasServiceQuotaCreate,
		Read:   resourceAwsServiceQuotasServiceQuotaRead,
		Update: resourceAwsServiceQuotasServiceQuotaUpdate,
		Delete: schema.Noop,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"quota_code": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"request_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"service_code": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"value": {
				Type:     schema.TypeFloat,
				Required: true,
			},
		},
	}
}

func resourceAwsServiceQuotasServiceQuotaCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).servicequotasconn

	quotaCode := d.Get("quota_code").(string)
	serviceCode := d.Get("service_code").(string)
	value := d.Get("value").(float64)

	d.SetId(fmt.Sprintf("%s/%s", serviceCode, quotaCode))

	input := &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(quotaCode),
		ServiceCode: aws.String(serviceCode),
	}

	output, err := conn.GetServiceQuota(input)

	if err != nil {
		return fmt.Errorf("error getting Service Quotas Service Quota (%s): %s", d.Id(), err)
	}

	if output == nil {
		return fmt.Errorf("error getting Service Quotas Service Quota (%s): empty result", d.Id())
	}

	if value > aws.Float64Value(output.Quota.Value) {
		input := &servicequotas.RequestServiceQuotaIncreaseInput{
			DesiredValue: aws.Float64(value),
			QuotaCode:    aws.String(quotaCode),
			ServiceCode:  aws.String(serviceCode),
		}

		output, err := conn.RequestServiceQuotaIncrease(input)

		if err != nil {
			return fmt.Errorf("error requesting Service Quota (%s) increase: %s", d.Id(), err)
		}

		if output == nil || output.RequestedQuota == nil {
			return fmt.Errorf("error requesting Service Quota (%s) increase: empty result", d.Id())
		}

		d.Set("request_id", output.RequestedQuota.Id)
	}

	return resourceAwsServiceQuotasServiceQuotaRead(d, meta)
}

func resourceAwsServiceQuotasServiceQuotaRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).servicequotasconn

	requestID := d.Get("request_id").(string)
	serviceCode, quotaCode, err := resourceAwsServiceQuotasServiceQuotaParseID(d.Id())

	if err != nil {
		return err
	}

	input := &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(quotaCode),
		ServiceCode: aws.String(serviceCode),
	}

	output, err := conn.GetServiceQuota(input)

	if err != nil {
		return fmt.Errorf("error getting Service Quotas Service Quota (%s): %s", d.Id(), err)
	}

	if output == nil {
		return fmt.Errorf("error getting Service Quotas Service Quota (%s): empty result", d.Id())
	}

	d.Set("quota_code", output.Quota.QuotaCode)
	d.Set("service_code", output.Quota.ServiceCode)
	d.Set("value", output.Quota.Value)

	if requestID != "" {
		input := &servicequotas.GetRequestedServiceQuotaChangeInput{
			RequestId: aws.String(requestID),
		}

		output, err := conn.GetRequestedServiceQuotaChange(input)

		if isAWSErr(err, servicequotas.ErrCodeNoSuchResourceException, "") {
			d.Set("request_id", "")
			d.Set("request_status", "")
			return nil
		}

		if err != nil {
			return fmt.Errorf("error getting Service Quotas Requested Service Quota Change (%s): %s", requestID, err)
		}

		if output != nil && output.RequestedQuota != nil {
			return fmt.Errorf("error getting Service Quotas Requested Service Quota Change (%s): empty result", requestID)
		}

		requestStatus := aws.StringValue(output.RequestedQuota.Status)
		d.Set("request_status", requestStatus)

		switch requestStatus {
		case servicequotas.RequestStatusApproved, servicequotas.RequestStatusCaseClosed, servicequotas.RequestStatusDenied:
			d.Set("request_id", "")
		case servicequotas.RequestStatusCaseOpened, servicequotas.RequestStatusPending:
			d.Set("value", output.RequestedQuota.DesiredValue)
		}
	}

	return nil
}

func resourceAwsServiceQuotasServiceQuotaUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).servicequotasconn

	value := d.Get("value").(float64)
	serviceCode, quotaCode, err := resourceAwsServiceQuotasServiceQuotaParseID(d.Id())

	if err != nil {
		return err
	}

	input := &servicequotas.RequestServiceQuotaIncreaseInput{
		DesiredValue: aws.Float64(value),
		QuotaCode:    aws.String(quotaCode),
		ServiceCode:  aws.String(serviceCode),
	}

	output, err := conn.RequestServiceQuotaIncrease(input)

	if err != nil {
		return fmt.Errorf("error requesting Service Quota (%s) increase: %s", d.Id(), err)
	}

	if output == nil || output.RequestedQuota == nil {
		return fmt.Errorf("error requesting Service Quota (%s) increase: empty result", d.Id())
	}

	d.Set("request_id", output.RequestedQuota.Id)

	return resourceAwsServiceQuotasServiceQuotaRead(d, meta)
}

func resourceAwsServiceQuotasServiceQuotaParseID(id string) (string, string, error) {
	parts := strings.SplitN(id, "/", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%s), expected SERVICE-CODE/QUOTA-CODE", id)
	}

	return parts[0], parts[1], nil
}
