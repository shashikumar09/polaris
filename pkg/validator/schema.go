package validator

import (
	"fmt"
	"sort"
	"strings"
	"math"
	"github.com/qri-io/jsonschema"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/datadog"
	"github.com/fairwindsops/polaris/pkg/arc_constants"
)

type schemaTestCase struct {
	Target           config.TargetKind
	Resource         kube.GenericResource
	IsInitContianer  bool
	Container        *corev1.Container
	ResourceProvider *kube.ResourceProvider
}


func resolveCheck(conf *config.Configuration, checkID string, test schemaTestCase) (*config.SchemaCheck, error) {
	if !conf.DisallowExemptions && hasExemptionAnnotation(test.Resource.ObjectMeta, checkID) {
		return nil, nil
	}
	check, ok := conf.CustomChecks[checkID]
	if !ok {
		check, ok = config.BuiltInChecks[checkID]
		if !ok {
			check, ok = conf.DynamicCustomChecks[checkID]
			if !ok {
				return nil, fmt.Errorf("Check %s not found", checkID)
			}
		}
	}
	containerName := ""
	if test.Container != nil {
		containerName = test.Container.Name
	}
	if !conf.IsActionable(checkID, test.Resource.ObjectMeta, containerName) {
		return nil, nil
	}
	if !check.IsActionable(test.Target, test.Resource.Kind, test.IsInitContianer) {
		return nil, nil
	}
	if _, ok := conf.DynamicCustomChecks[checkID]; !ok {

		checkPtr, err := check.TemplateForResource(test.Resource.Resource.Object)
		if err != nil {
			return nil, err
		}
		return checkPtr, nil
	} else {
		return  &check, nil
	}
}

func makeResult(conf *config.Configuration, check *config.SchemaCheck, passes bool, issues []jsonschema.ValError, data ResourceInfo) ResultMessage {
	details := []string{}
	for _, issue := range issues {
		details = append(details, issue.Message)
	}
	result := ResultMessage{
		ID:       check.ID,
		Severity: conf.Checks[check.ID] ,
		Category: check.Category,
		Success:  passes,
		Data: data, //applicable for only wastageCost check for remainig checks it will be zero
		// FIXME: need to fix the tests before adding this back
		//Details: details,
	}
	if passes {
		result.Message = check.SuccessMessage
	} else {
		result.Message = check.FailureMessage
	}
	return result
}

func makeDynamicResult(conf *config.Configuration, check *config.SchemaCheck, passes bool, data ResourceInfo) ResultMessage {
	result := ResultMessage{
		ID:       check.ID,
		Severity: conf.Checks[check.ID] ,
		Category: check.Category,
		Success:  passes,
		Data: data, //applicable for only wastageCost check for remainig checks it will be zero
		// FIXME: need to fix the tests before adding this back
		//Details: details,
	}
	if passes {
		result.Message = check.SuccessMessage
	} else {
		result.Message = check.FailureMessage
	}
	return result
}


const exemptionAnnotationKey = "polaris.fairwinds.com/exempt"

const exemptionAnnotationPattern = "polaris.fairwinds.com/%s-exempt"

func hasExemptionAnnotation(objMeta metaV1.Object, checkID string) bool {
	annot := objMeta.GetAnnotations()
	val := annot[exemptionAnnotationKey]
	if strings.ToLower(val) == "true" {
		return true
	}
	checkKey := fmt.Sprintf(exemptionAnnotationPattern, checkID)
	val = annot[checkKey]
	if strings.ToLower(val) == "true" {
		return true
	}
	return false
}

// ApplyAllSchemaChecksToResourceProvider applies all available checks to a ResourceProvider
func ApplyAllSchemaChecksToResourceProvider(conf *config.Configuration, resourceProvider *kube.ResourceProvider) ([]Result, error) {
	results := []Result{}
	for _, resources := range resourceProvider.Resources {
		kindResults, err := ApplyAllSchemaChecksToAllResources(conf, resourceProvider, resources)
		if err != nil {
			return results, err
		}
		results = append(results, kindResults...)
	}
	return results, nil
}

// ApplyAllSchemaChecksToAllResources applies available checks to a list of resources
func ApplyAllSchemaChecksToAllResources(conf *config.Configuration, resourceProvider *kube.ResourceProvider, resources []kube.GenericResource) ([]Result, error) {
	results := []Result{}
	for _, resource := range resources {
		result, err := ApplyAllSchemaChecks(conf, resourceProvider, resource)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

// ApplyAllSchemaChecks applies available checks to a single resource
func ApplyAllSchemaChecks(conf *config.Configuration, resourceProvider *kube.ResourceProvider, resource kube.GenericResource) (Result, error) {
	if resource.PodSpec == nil {
		return applyNonControllerSchemaChecks(conf, resourceProvider, resource)
	}
	return applyControllerSchemaChecks(conf, resourceProvider, resource)
}

func applyNonControllerSchemaChecks(conf *config.Configuration, resourceProvider *kube.ResourceProvider, resource kube.GenericResource) (Result, error) {
	var appLabel string
        if val, ok := resource.ObjectMeta.GetLabels()["app.kubernetes.io/name"]; ok {
                appLabel = val
        } else {
                fmt.Println("Label not found for resource: ", resource.ObjectMeta.GetName())
        }

        finalResult := Result{
                Kind:      resource.Kind,
                Name:      resource.ObjectMeta.GetName(),
                Namespace: resource.ObjectMeta.GetNamespace(),
                AppLabel:     appLabel,
        }
	resultSet, err := applyTopLevelSchemaChecks(conf, resourceProvider, resource, false)
	finalResult.Results = resultSet
	return finalResult, err
}

func applyControllerSchemaChecks(conf *config.Configuration, resourceProvider *kube.ResourceProvider, resource kube.GenericResource) (Result, error) {
	var appLabel string
        if val, ok := resource.ObjectMeta.GetLabels()["app.kubernetes.io/name"]; ok {
                appLabel = val
        } else {
                fmt.Println("label not found for resource: ", resource.ObjectMeta.GetName())
        }

        finalResult := Result{
                Kind:      resource.Kind,
                Name:      resource.ObjectMeta.GetName(),
                Namespace: resource.ObjectMeta.GetNamespace(),
                AppLabel:     appLabel,
        }
	resultSet, err := applyTopLevelSchemaChecks(conf, resourceProvider, resource, true)
	if err != nil {
		return finalResult, err
	}
	finalResult.Results = resultSet

	podRS, err := applyPodSchemaChecks(conf, resourceProvider, resource)
	if err != nil {
		return finalResult, err
	}
	podRes := PodResult{
		Results:          podRS,
		ContainerResults: []ContainerResult{},
	}
	finalResult.PodResult = &podRes

	for _, container := range resource.PodSpec.InitContainers {
		results, err := applyContainerSchemaChecks(conf, resourceProvider, resource, &container, true)
		if err != nil {
			return finalResult, err
		}
		cRes := ContainerResult{
			Name:    container.Name,
			Results: results,
		}
		podRes.ContainerResults = append(podRes.ContainerResults, cRes)
	}
	for _, container := range resource.PodSpec.Containers {
		results, err := applyContainerSchemaChecks(conf, resourceProvider, resource, &container, false)
		if err != nil {
			return finalResult, err
		}
		cRes := ContainerResult{
			Name:    container.Name,
			Results: results,
		}
		podRes.ContainerResults = append(podRes.ContainerResults, cRes)
	}

	return finalResult, nil
}

func applyTopLevelSchemaChecks(conf *config.Configuration, resources *kube.ResourceProvider, res kube.GenericResource, isController bool) (ResultSet, error) {
	test := schemaTestCase{
		ResourceProvider: resources,
		Resource:         res,
	}
	if isController {
		test.Target = config.TargetController
	}
	return applySchemaChecks(conf, test)
}

func applyPodSchemaChecks(conf *config.Configuration, resources *kube.ResourceProvider, controller kube.GenericResource) (ResultSet, error) {
	test := schemaTestCase{
		Target:           config.TargetPod,
		ResourceProvider: resources,
		Resource:         controller,
	}
	return applySchemaChecks(conf, test)
}

func applyContainerSchemaChecks(conf *config.Configuration, resources *kube.ResourceProvider, controller kube.GenericResource, container *corev1.Container, isInit bool) (ResultSet, error) {
	test := schemaTestCase{
		Target:           config.TargetContainer,
		ResourceProvider: resources,
		Resource:         controller,
		Container:        container,
		IsInitContianer:  isInit,
	}
	return applySchemaChecks(conf, test)
}

func applySchemaChecks(conf *config.Configuration, test schemaTestCase) (ResultSet, error) {
	results := ResultSet{}
	checkIDs := getSortedKeys(conf.Checks)
	for _, checkID := range checkIDs {
		if _, ok := conf.DynamicCustomChecks[checkID]; ok {
			result, err := applyDynamicSchemaCheck(conf, checkID, test)
			if err != nil {
				return results, err
			}
		
			if result != nil {
				results[checkID] = *result
			}
		} else {
			result, err := applySchemaCheck(conf, checkID, test);
			if err != nil {
				return results, err
			}

			if result != nil {
				results[checkID] = *result
			}
		}
		}
	
	return results, nil
}


func HandleHPALimitsCheck(check *config.SchemaCheck, checkID string, test schemaTestCase) (bool, []string, error){
	
	var upperClusterLimits  datadog.HPALimits;
	var lowerClusterLimits datadog.HPALimits
	capacityLimit, ok := check.Schema["capacityLimit"].(float64);
	if !ok {
	   return false, nil,fmt.Errorf("Capacity Limit not found for schema", checkID)
	}
	var cluster string
	cluster, ok = check.Schema["cluster"].(string) 
	if !ok {
		return false, nil, fmt.Errorf("Cluster not found on HPA limits check schema", checkID)
	}
	upperCluster, upperClusterOk := check.Schema["upperCluster"].(string)
	lowerCluster, lowerClusterOk := check.Schema["lowerCluster"].(string)
	if !upperClusterOk && !lowerClusterOk {
		return false, nil, fmt.Errorf("Lower/Upper cluster not found on HPA limits check schema", checkID)
	}
	if arc_constants.MACHINE_STABILITY == arc_constants.DEV || arc_constants.MACHINE_STABILITY == arc_constants.QA  {
		upperClusterLimits = datadog.GetHPALimits(test.Resource.ObjectMeta.GetName(), upperCluster)
		if upperClusterLimits.Max == 0 {
			fmt.Println("Instance not present in upper cluster", test.Resource.ObjectMeta.GetName(), test.Resource.ObjectMeta.GetNamespace())
			return false, []string{"Instance not present in upper cluster, Ignoring the test"} , nil
		}
	} else if arc_constants.MACHINE_STABILITY == arc_constants.UAT {
		upperClusterLimits = datadog.GetHPALimitsForDeployment(test.Resource.ObjectMeta.GetName(), strings.ReplaceAll(test.Resource.ObjectMeta.GetNamespace(), arc_constants.UAT, ""), upperCluster)
		lowerClusterLimits = datadog.GetHPALimits(test.Resource.ObjectMeta.GetName(), lowerCluster)
	}
	actualLimits  := datadog.GetHPALimitsForDeployment(test.Resource.ObjectMeta.GetName(), test.Resource.ObjectMeta.GetNamespace(), cluster)
	if  math.Ceil(actualLimits.Max) <= math.Ceil(upperClusterLimits.Max * capacityLimit) &&  math.Ceil(actualLimits.Max) >= math.Ceil(lowerClusterLimits.Max * 1/capacityLimit) 	{
		return true, nil, nil
	} else {
		return false, nil, nil
	}
}

func HandleWastageCostCheck(check *config.SchemaCheck, checkID string, test schemaTestCase) (bool, ResourceInfo,   []string, error) {
	var resourceInfo ResourceInfo
	var totalWastageCost int
	var wastageCost datadog.ResourceCost
	var allowedWastage float64;
	var err error
	cluster, ok := check.Schema["cluster"].(string) 
	if !ok {
		return false, resourceInfo, nil, fmt.Errorf("Cluster not found on HPA limits check schema", checkID)
	}
	resourceDetails := datadog.GetResourceDetails(test.Resource.Kind, test.Resource.ObjectMeta.GetName(), test.Resource.ObjectMeta.GetNamespace(), cluster)
	totalWastageCost, wastageCost, err = GetWastageCost(check, &resourceDetails)
	if err != nil{
		return false, resourceInfo, nil, fmt.Errorf("CPU/Memory cost per unit not found in schema")
	}
	allowedWastage, ok = check.Schema["allowedWastage"].(float64);
	if !ok {
	   return false, resourceInfo, nil, fmt.Errorf("Allowed wastage not found for schema ", checkID) 
	}
	resourceInfo.Name = test.Resource.ObjectMeta.GetName()
	resourceInfo.ResourceDetails = resourceDetails
	resourceInfo.WastageCost = wastageCost
	resourceInfo.TotalWastageCost = totalWastageCost
	resourceInfo.AllowedWastage = int(allowedWastage)
	if totalWastageCost > int(allowedWastage) {
		message := "Wastage cost is " + fmt.Sprintf("%d", totalWastageCost) + "$/mo"
		check.FailureMessage = message
		return false, resourceInfo,  nil, nil
	} else {
		message := "Wastage cost is within limit"
		check.SuccessMessage = message
		return true, resourceInfo, nil, nil
	}
}

func HandleResourceLimitsCheck(check *config.SchemaCheck, checkID string, test schemaTestCase) (bool, []string, error) {
	//var actualLimits float64;
	var expectedResourceLimits datadog.ResourceLimits
	capacityLimit, ok := check.Schema["capacityLimit"].(float64);
	if !ok {
	   return false, nil,fmt.Errorf("capacity Limit not found for schema", checkID)
	}		
	var cluster string
	var upperCluster string
	cluster, ok = check.Schema["cluster"].(string)  
	if !ok {
		return false, nil, fmt.Errorf("Cluster not found on HPA limits check schema", checkID)
	}
	upperCluster, ok = check.Schema["upperCluster"].(string)
	if !ok {
		return false, nil, fmt.Errorf("Upper cluster not found on HPA limits check schema", checkID)
	}
	if arc_constants.MACHINE_STABILITY == arc_constants.DEV || arc_constants.MACHINE_STABILITY == arc_constants.QA  {
		expectedResourceLimits = datadog.GetResourceLimits(test.Resource.ObjectMeta.GetName(), upperCluster)
		if expectedResourceLimits.CPU == 0 && expectedResourceLimits.Memory == 0 {
			return false, []string{"No deployments found in upper cluster"}, nil
		}
	} else  {
		expectedResourceLimits = datadog.GetResourceLimitsForDeployment(test.Resource.ObjectMeta.GetName(), strings.ReplaceAll(test.Resource.ObjectMeta.GetNamespace(), "uat", ""), upperCluster)
	} 
	actualResourceLimits  := datadog.GetResourceLimitsForDeployment(test.Resource.ObjectMeta.GetName(), test.Resource.ObjectMeta.GetNamespace(), cluster)
	if int(actualResourceLimits.CPU) > int(expectedResourceLimits.CPU)*int(capacityLimit) && int(actualResourceLimits.Memory) > int(expectedResourceLimits.Memory)*int(capacityLimit) {
		check.FailureMessage = "CPU/Memory limits are not within range (CPU: " + fmt.Sprintf("%0.2f", actualResourceLimits.CPU) + " Memory: " + fmt.Sprintf("%0.2f", actualResourceLimits.Memory) + ")"
		return false, nil, nil
	} else {
		return true, nil,nil
	}
}

func applyDynamicSchemaCheck(conf *config.Configuration, checkID string, test schemaTestCase) (*ResultMessage, error) {
	// Will perform DynamicSchemaChecks

	//check, err := resolveCheck(conf, checkID, test)
	check, err := resolveCheck(conf, checkID, test)
	if err != nil {
		return nil, err
	} else if check == nil {
		return nil, nil
	}
	var data  ResourceInfo
	var passes bool
	var issues []string
	if checkID == "HPALimits" {
		if test.Resource.Kind == "Deployment" {
			passes, issues, err = HandleHPALimitsCheck(check, checkID, test)
			if err != nil {
				return nil, err
			} else if issues != nil {
				return nil, nil
			}
 		}
	} else if checkID == "WastageCost" {
		passes, data, issues, err = HandleWastageCostCheck(check, checkID, test)
		if err != nil {
			return nil, err
		}
	} else if checkID == "ResourceLimits" {
		passes, issues, err = HandleResourceLimitsCheck(check, checkID, test)
		if err != nil {
			return nil, err
		} else if issues != nil {
			return nil, nil
		}
	} else {
		return nil, nil
	}
	result := makeDynamicResult(conf, check, passes, data)
	return &result, nil
}

func applySchemaCheck(conf *config.Configuration, checkID string, test schemaTestCase) (*ResultMessage, error) {
	check, err := resolveCheck(conf, checkID, test)
	if err != nil {
		return nil, err
	} else if check == nil {
		return nil, nil
	}
	var data ResourceInfo
	var passes bool
	var issues []jsonschema.ValError
	if check.SchemaTarget != "" {
		if check.SchemaTarget == config.TargetPod && check.Target == config.TargetContainer {
			podCopy := *test.Resource.PodSpec
			podCopy.InitContainers = []corev1.Container{}
			podCopy.Containers = []corev1.Container{*test.Container}
			passes, issues, err = check.CheckPod(&podCopy)
		} else {
			return nil, fmt.Errorf("Unknown combination of target (%s) and schema target (%s)", check.Target, check.SchemaTarget)
		}
	} else if check.Target == config.TargetPod {
		passes, issues, err = check.CheckPod(test.Resource.PodSpec)
	} else if check.Target == config.TargetContainer {
		passes, issues, err = check.CheckContainer(test.Container)
	} else {
		passes, issues, err = check.CheckObject(test.Resource.Resource.Object)
	}
	if err != nil {
		return nil, err
	}
	for groupkind := range check.AdditionalValidators {
		if !passes {
			break
		}
		resources := test.ResourceProvider.Resources[groupkind]
		namespace := test.Resource.ObjectMeta.GetNamespace()
		if test.Resource.Kind == "Namespace" {
			namespace = test.Resource.ObjectMeta.GetName()
		}
		resources = funk.Filter(resources, func(res kube.GenericResource) bool {
			return res.ObjectMeta.GetNamespace() == namespace
		}).([]kube.GenericResource)
		objects := funk.Map(resources, func(res kube.GenericResource) interface{} {
			return res.Resource.Object
		}).([]interface{})
		passes, err = check.CheckAdditionalObjects(groupkind, objects)
		if err != nil {
			return nil, err
		}
	}
	result := makeResult(conf, check, passes, issues, data)
	return &result, nil
}

func getSortedKeys(m map[string]config.Severity) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func GetWastageCostOverview(conf *config.Configuration,resourceProvider *kube.ResourceProvider, results []Result) datadog.WastageCostOverview {

    var wastageCostOverview datadog.WastageCostOverview
	wastageCostOverview.Namespace = make(map[string]int)

    for _, namespace := range resourceProvider.Namespaces {
            wastageCostOverview.Namespace[namespace.ObjectMeta.GetName()] = GetWastageCostOnNamespace(results, namespace.ObjectMeta.GetName())
    }
	clusterWastageCost := GetWastageCostOnCluster(wastageCostOverview)
	wastageCostOverview.Value = clusterWastageCost
	wastageCostOverview.FormattedValue = arc_constants.NearestThousandFormat(float64(clusterWastageCost))
	return wastageCostOverview
}

func GetWastageCostOnNamespace(results []Result, namespace string) int {

	var wastageCostOnNamespace int
	for _, result := range results {
		if result.Namespace == namespace {
			for _, check:= range result.Results{
				if check.ID == "WastageCost" {
					wastageCostOnNamespace = wastageCostOnNamespace + check.Data.TotalWastageCost
				}
			}
		}
	}
	return wastageCostOnNamespace

}

func GetWastageCostOnCluster(wastageCostOverview datadog.WastageCostOverview) int {

	var wastageCostOnCluster int
	for  _, wastageCost := range wastageCostOverview.Namespace {
		wastageCostOnCluster = wastageCostOnCluster + wastageCost

	}
	return wastageCostOnCluster
}



func GetWastageCost(check *config.SchemaCheck, resourceDetails *datadog.ResourceDetails) (int, datadog.ResourceCost, error) {
	var ok bool
	var ResourceCostPerUnit datadog.ResourceCost;
	var WastageCost datadog.ResourceCost;
	var totalWastageCost int;
	resourceWastage := resourceDetails.Wastage
	ResourceCostPerUnit.CPU, ok = check.Schema["cpuCostPerUnit"].(float64); 
	if !ok {
 	   return totalWastageCost, WastageCost, fmt.Errorf("CPU cost pet unit not found for schema ", check.ID) 
	}
	ResourceCostPerUnit.Memory, ok = check.Schema["memoryCostPerUnit"].(float64);
	if !ok {
 	   return totalWastageCost, WastageCost, fmt.Errorf("Memory cost pet unit not found for schema ", check.ID) 
	}
	WastageCost.CPU = (resourceWastage.CPU * ResourceCostPerUnit.CPU)
	WastageCost.Memory = (resourceWastage.Memory * ResourceCostPerUnit.Memory)
	totalWastageCost = int(WastageCost.CPU + WastageCost.Memory)
	return totalWastageCost, WastageCost, nil

}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

