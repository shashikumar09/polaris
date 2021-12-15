package datadog

import (
    "context"
    "fmt"
    "os"
    "time"
    "log"
    "io/ioutil"
    "encoding/json"
    datadogClient "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
    "github.com/fairwindsops/polaris/pkg/arc_constants"
)


type QueryResponse struct {
	Status string  `json:"status"`
    Series  []struct {
        Metric string `json:"metric"`
        TagSet []string `json:"tag_set"`
        PointList [][]float64 `json:"pointlist"`
        Unit []struct {
            ScaleFactor float64 `json:"scale_factor"`
        } `json:"unit"`
        } `json:"series"`
    
}

type WastageCostOverview struct {
	Value int
	FormattedValue string
	Namespace map[string]int
}

type HPALimits struct {
    Min float64
    Max float64
}

type ResourceLimits struct {
    CPU float64
    Memory float64
}

type ResourceRequests struct {
    CPU float64
    Memory float64
}

type ResourceCost struct {
    CPU float64
    Memory float64
}

type ResourceUsage struct {
    CPU float64
    Memory float64
}

type ResourceDetails struct {
    Requests ResourceRequests
    Usage ResourceUsage
    Wastage ResourceUsage
}



var (
    HPALimitsQuery string = "avg:kubernetes_state.hpa.max_replicas{*}by{hpa,kube_namespace,cluster_name}"
    ResourcesLimitsForDeploymentQuery  string = "avg:kubernetes.cpu.limits{*}by{kube_deployment,kube_namespace,cluster_name},avg:kubernetes.memory.limits{*}by{kube_deployment,kube_namespace,cluster_name}";
    ResourcesLimitsQuery  string = "avg:kubernetes.cpu.limits{*}by{kube_deployment,cluster_name},avg:kubernetes.memory.limits{*}by{kube_deployment,cluster_name}";
    ReplicasCountForDeploymentQuery string = "avg:kubernetes_state.deployment.replicas{*}by{kube_deployment,kube_namespace,cluster_name}";
    ReplicasCountQuery string = "avg:kubernetes_state.deployment.replicas{*}by{kube_deployment,cluster_name}";
    ResourceRequestsQuery string = `sum:kubernetes.cpu.requests{*}by{kube_deployment,kube_namespace,cluster_name},sum:kubernetes.memory.requests{*}by{kube_deployment,kube_namespace,cluster_name},sum:kubernetes.cpu.requests{*}by{kube_daemon_set,kube_namespace,cluster_name},sum:kubernetes.memory.requests{*}by{kube_daemon_set,kube_namespace,cluster_name},sum:kubernetes.cpu.requests{*}by{kube_stateful_set,kube_namespace,cluster_name},sum:kubernetes.memory.requests{*}by{kube_stateful_set,kube_namespace,cluster_name}`
    ResourceUsageQuery string = `sum:kubernetes.cpu.usage.total{*}by{kube_deployment,kube_namespace,cluster_name},sum:kubernetes.memory.usage{*}by{kube_deployment,kube_namespace,cluster_name},sum:kubernetes.cpu.usage.total{*}by{kube_daemon_set,kube_namespace,cluster_name},sum:kubernetes.memory.usage{*}by{kube_daemon_set,kube_namespace,cluster_name},sum:kubernetes.cpu.usage.total{*}by{kube_stateful_set,kube_namespace,cluster_name},sum:kubernetes.memory.usage{*}by{kube_stateful_set,kube_namespace,cluster_name}`

    ResourceLimitsForDeployment QueryResponse;
    ResourceLimitsData QueryResponse;
    HPALimitsForDeployment QueryResponse;
    ReplicasCountForDeployment QueryResponse;
    ReplicasCount QueryResponse;
    ResourceRequestsData QueryResponse;
    ResourceUsageData QueryResponse;
    DDClientKeys arc_constants.VaultResponse;
)

var datadogKindDict map[string]string
func init() {
   datadogKindDict = make(map[string]string) 
    datadogKindDict["Deployment"] = "kube_deployment:"
    datadogKindDict["DaemonSet"] =  "kube_daemon_set:"
    datadogKindDict["StatefulSet"] =  "kube_stateful_set:"
    DDClientKeys = arc_constants.GetDDSecretsFromVault()
}

func InitializeDatadogQueryResponses() {
    ResourceLimitsForDeployment = queryTSMetricsFromDatadog(ResourcesLimitsForDeploymentQuery)
    ResourceLimitsData = queryTSMetricsFromDatadog(ResourcesLimitsQuery)
    HPALimitsForDeployment = queryTSMetricsFromDatadog(HPALimitsQuery)
    ReplicasCountForDeployment = queryTSMetricsFromDatadog(ReplicasCountForDeploymentQuery)
    ReplicasCount = queryTSMetricsFromDatadog(ReplicasCountQuery)
    ResourceRequestsData = queryTSMetricsFromDatadog(ResourceRequestsQuery)
    ResourceUsageData = queryTSMetricsFromDatadog(ResourceUsageQuery)
}

func queryTSMetricsFromDatadog(query string) QueryResponse {
    ctx := context.WithValue(
        context.Background(),
        datadogClient.ContextAPIKeys,
        map[string]datadogClient.APIKey{
            "apiKeyAuth": {
                Key: DDClientKeys.Data.ConnectionProperties.ApiKey,
            },
            "appKeyAuth": {
                Key: DDClientKeys.Data.ConnectionProperties.AppKey,
            },
        },
    )
    fmt.Println(query)
    configuration := datadogClient.NewConfiguration()
    apiClient := datadogClient.NewAPIClient(configuration)
    now := time.Now()
    from := now.AddDate(0, -1, 0).Unix()
    to := now.Unix()
    fmt.Println("from", from)
    fmt.Println("to", to)
    resp, r, err := apiClient.MetricsApi.QueryMetrics(ctx, from, to, query)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `MetricsApi.ListTagsByMetricName`: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `QueryMetrics`: 
    json.MarshalIndent(resp, "", "  ")
    //responseContent, _ := json.MarshalIndent(resp, "", "  ")
    //fmt.Fprintf(os.Stdout, "Response from MetricsApi.ListTagsByMetricName:\n%s\n", responseContent)
    var queryResponse1 QueryResponse
    defer r.Body.Close()
    bodyBytes, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Fatal(err)
    }
    err1 := json.Unmarshal([]byte(bodyBytes), &queryResponse1)
    if err1 != nil {
	    log.Fatal(err1)
	   }

    //fmt.Println(queryResponse1)
    return queryResponse1

}


func GetResourceLimitsForDeployment(deployment string, namespace string, cluster string) ResourceLimits {

    var resourceLimits ResourceLimits;
    var CPUMetric string = "kubernetes.cpu.limits"
    var MemoryMetric string = "kubernetes.memory.limits"
    for _,i:= range ResourceLimitsForDeployment.Series {
	if i.TagSet[2] == "kube_namespace:" + namespace  && i.TagSet[1] == "kube_deployment:" + deployment && i.TagSet[0] == "cluster_name:" + cluster {
            if i.Metric == CPUMetric {
                resourceLimits.CPU = i.PointList[len(i.PointList)-1][1] * i.Unit[0].ScaleFactor //To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format

            }else if i.Metric == MemoryMetric {

                resourceLimits.Memory = i.PointList[len(i.PointList)-1][1]/1073741824 //To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format

           }
        }
    }
    return resourceLimits
    
}
func GetResourceLimits(deployment string,  cluster string) ResourceLimits {

    var resourceLimits ResourceLimits;
    var CPUMetric string = "kubernetes.cpu.limits"
    var MemoryMetric string = "kubernetes.memory.limits"
    for _,i:= range ResourceLimitsData.Series {
	if  i.TagSet[1] == "kube_deployment:" + deployment && i.TagSet[0] == "cluster_name:" + cluster {
            if i.Metric == CPUMetric {
                resourceLimits.CPU = i.PointList[len(i.PointList)-1][1]  * i.Unit[0].ScaleFactor//To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format

            }else if i.Metric == MemoryMetric {

                resourceLimits.Memory = i.PointList[len(i.PointList)-1][1]/1073741824 //To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format

           }
        }
    }
    return resourceLimits
    
}  


func isDeploymentContainsHPA(deployment string, namespace string, cluster string) bool{

    for _, i:= range HPALimitsForDeployment.Series {
	    if i.TagSet[2] == "kube_namespace:" + namespace && i.TagSet[1] == "hpa:hpa-" + deployment && i.TagSet[0] == "cluster_name:" + cluster {
            return true
        }
        
	}
    return false
    
} 

func FetchHPALimitForDeployment(deployment string, namespace string, cluster string) float64 {
   
    var limit float64;
    for _, i:= range HPALimitsForDeployment.Series {
	    if i.TagSet[2] == "kube_namespace:" + namespace && i.TagSet[1] == "hpa:hpa-" + deployment && i.TagSet[0] == "cluster_name:" + cluster {
            limit = i.PointList[len(i.PointList)-1][1] //To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format

	}
    }
    return limit
}

func FetchRepicasCountForDeployment(deployment string, namespace string, cluster string) float64 {
    
    var replicaCount float64;
    for _, i:= range ReplicasCountForDeployment.Series {
	    if i.TagSet[2] == "kube_namespace:" + namespace  && i.TagSet[1] == "kube_deployment:" + deployment && i.TagSet[0] == "cluster_name:" + cluster{
	    replicaCount =  i.PointList[len(i.PointList)-1][1] ///To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format
	    break;
        }
    }
    return replicaCount


}

func GetHPALimitsForDeployment(deployment string, namespace string, cluster string) HPALimits{
    //It will check whether HPA defined or not. If defined it will return min/max limits. else it will return min as 1 max as replicas configured for deplyoment
    var HPALimits HPALimits;
    HPALimits.Min = 1
    if isDeploymentContainsHPA(deployment, namespace, cluster) {
	limit := FetchHPALimitForDeployment(deployment, namespace, cluster)
        HPALimits.Max = limit
        return HPALimits

     } else {
        limit := FetchRepicasCountForDeployment(deployment, namespace, cluster)
        HPALimits.Max = limit
	    return HPALimits
    }

}

//To get the average of entire upper cluster(Inlcuding all namespaces where given deployment running)
func GetHPALimits(deployment string, cluster string) HPALimits {
    var HPALimits HPALimits;
    HPALimits.Min = 1
    for _, i:= range ReplicasCount.Series {
	    if i.TagSet[1] == "kube_deployment:" + deployment && i.TagSet[0] == "cluster_name:" + cluster{
	    HPALimits.Max = i.PointList[len(i.PointList)-1][1] //To get the latest timestamp value. Pointlist stored in [<timestamp> <value>] format
	    break;
        }
    }
    return HPALimits
}



func getAvgOfPolinListData(pointList [][]float64) float64 {

	var dataPoints []float64
	for _, i := range pointList{
		if i != nil {
			dataPoints = append(dataPoints, i[1])
		}
	}
	sum := 0.0

    for i := 0; i < len(dataPoints); i++ {
        sum += (dataPoints[i])
	}
	return sum/float64(len(dataPoints))
}

func getResourceRequests(kind string, resource string, namespace string, cluster string) ResourceRequests {

    var resourceRequests ResourceRequests;
    var CPUMetric string = "kubernetes.cpu.requests";
    var MemoryMetric string = "kubernetes.memory.requests";
    for _,i:= range ResourceRequestsData.Series {
	    if i.TagSet[2] == "kube_namespace:" + namespace  && i.TagSet[1] == datadogKindDict[kind] + resource && i.TagSet[0] == "cluster_name:"+ cluster {
            	if i.Metric == CPUMetric {
		resourceRequests.CPU = getAvgOfPolinListData(i.PointList) * i.Unit[0].ScaleFactor //Pointlist stored in [<timestamp> <value>] format

            }else if i.Metric == MemoryMetric {

                resourceRequests.Memory = getAvgOfPolinListData(i.PointList)/1073741824 //Pointlist stored in [<timestamp> <value>] format

           }
        }
    }
    return resourceRequests
    
} 

func getResourceUsage(kind string, resource string, namespace string, cluster string) ResourceUsage{
    var resourceUsage ResourceUsage;
    var CPUMetric string = "kubernetes.cpu.usage.total" 
    var MemoryMetric string = "kubernetes.memory.usage"
    for _,i:= range ResourceUsageData.Series {
	    if i.TagSet[2] == "kube_namespace:" + namespace  && i.TagSet[1] == datadogKindDict[kind] + resource && i.TagSet[0] == "cluster_name:" + cluster {
            if i.Metric == CPUMetric {
                resourceUsage.CPU = getAvgOfPolinListData(i.PointList) * i.Unit[0].ScaleFactor //Pointlist stored in [<timestamp> <value>] format

            }else if i.Metric == MemoryMetric {

                resourceUsage.Memory = getAvgOfPolinListData(i.PointList)/1073741824 //Pointlist stored in [<timestamp> <value>] format

           }
        }
    }
    return resourceUsage
    
}

func GetResourceDetails(kind string, resource string, namespace string, cluster string) ResourceDetails {
     	var resourceDetails ResourceDetails
	var resourceWastage ResourceUsage
	resourceRequests := getResourceRequests(kind, resource, namespace, cluster)
	resourceUsage := getResourceUsage(kind, resource, namespace, cluster)
	resourceDetails.Requests = resourceRequests
	resourceDetails.Usage = resourceUsage	
	resourceWastage.CPU = (resourceRequests.CPU - resourceUsage.CPU)
	resourceWastage.Memory = resourceRequests.Memory - resourceUsage.Memory
	resourceDetails.Wastage = resourceWastage
	return resourceDetails
} 
