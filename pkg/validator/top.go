package validator


import (
        "sort"
	"fmt"
)

type TopScorers struct {
        Namespaces  []kv
        Resources  []kv

}
type TopWastageCost struct {
	Namespaces []kvint
	Resources []kvint
}

type kv struct {
     Key   string
     Value uint
}

type kvint struct {
	Key string
	Value int
}

func (a AuditData) GetUniqueResources()  []string {
        var totalResources  []string
        for _, res := range a.Results {
                if !stringInSlice(res.Name, totalResources) {
                        totalResources = append(totalResources, res.Name)
                }
        }
        return totalResources
}

func (a AuditData) GetTopScorers() TopScorers {

        topScorers := TopScorers{}
        topScorers.Namespaces = a.GetTopNamespacesByScore()
        topScorers.Resources = a.GetTopResourcesByScore()
        return topScorers
}

func (a AuditData) GetTopWastageCost() TopWastageCost {

	topWastageCost := TopWastageCost{}
	topWastageCost.Namespaces = a.GetTopNamespacesByWastageCost()
	topWastageCost.Resources = a.GetTopResourcesByWastageCost()
	return topWastageCost
}

func (a AuditData) GetTopNamespacesByScore() []kv {
        var totalNamespaces map[string]uint
        totalNamespaces = make(map[string]uint)
        for ns, nsResults := range a.GetResultsByNamespace() {
                var totalScore uint
                var resourceCount uint
                for _, res := range nsResults {
                        totalScore += uint(res.GetSummary().GetScore())
                        resourceCount += 1
                }
                totalNamespaces[ns] =  uint(totalScore/resourceCount)
        }
	fmt.Println(totalNamespaces)
        var sortedNamespaces []kv
        for k, v := range totalNamespaces {
                sortedNamespaces = append(sortedNamespaces, kv{k, v})
        }
        sort.Slice(sortedNamespaces, func(i, j int) bool {
                return sortedNamespaces[i].Value > sortedNamespaces[j].Value
        })
	fmt.Println(sortedNamespaces)
        return sortedNamespaces[0:5]
}
func sum(array []uint) uint {
        var result uint
	for _, v := range array {
                result += v
	}
 return result
}

func sumOfInt(array []int) int {
	var result int
	for _,v := range array {
		result += v
	}
	return result
}

func isResourceAssociatedWithDeployment(nsResults []*Result, resource string) bool {

        for _, i := range nsResults {
                if i.Name == resource && i.Kind == "Deployment" {
                        return true
	                }
        }
        return false
}

func (a AuditData) GetTopResourcesByScore() []kv {
        var FinalMap map[string][]uint
        FinalMap = make(map[string][]uint)
        totalResources := make(map[string]uint)
        for _, nsResults := range a.GetResultsByNamespace()  {
                for  _, res := range nsResults {
                        var appLabel string
                        if len(res.AppLabel) == 0 {
                                appLabel = res.Name
                        } else {
                                appLabel = res.AppLabel
                        }
                        score := uint(res.GetSummary().GetScore())
                        if _, ok := FinalMap[appLabel]; ok {
                                FinalMap[appLabel] = append(FinalMap[appLabel], score)
                        } else if isResourceAssociatedWithDeployment(nsResults, appLabel) {
                                FinalMap[appLabel] = []uint{score}
                        }
                }
        }
        for key, val := range FinalMap {
                totalResources[key] = sum(val)/uint(len(val))
        }
        var sortedResources []kv
        for k, v := range totalResources {
                sortedResources = append(sortedResources, kv{k, v})
        }
        sort.Slice(sortedResources, func(i, j int) bool {
                return sortedResources[i].Value > sortedResources[j].Value
        })
        return sortedResources[0:5]
}

func (a AuditData) GetTopNamespacesByWastageCost() []kvint {
	var sortedNamespaces []kvint
 	for k, v := range a.WastageCostOverview.Namespace {
		sortedNamespaces = append(sortedNamespaces, kvint{k, v})
        }
	sort.Slice(sortedNamespaces, func(i, j int) bool {
                return sortedNamespaces[i].Value > sortedNamespaces[j].Value
        })
        return sortedNamespaces[0:5]
}

func (a AuditData) GetTopResourcesByWastageCost() []kvint {
	FinalMap := make(map[string][]int)
	for _, result := range a.Results {
		for _, check:= range result.Results{
			if check.ID == "WastageCost" {
				if _, ok := FinalMap[result.Name]; ok{
					FinalMap[result.Name] = append(FinalMap[result.Name], check.Data.TotalWastageCost)
				} else {
					FinalMap[result.Name] = []int{check.Data.TotalWastageCost}
				}
			}
		}
	}
	totalResources := make(map[string]int)
        for key, val := range FinalMap {
                totalResources[key] = sumOfInt(val)/len(val)
        }
        var sortedResources []kvint
        for k, v := range totalResources {
                sortedResources = append(sortedResources, kvint{k, v})
        }
        sort.Slice(sortedResources, func(i, j int) bool {
                return sortedResources[i].Value > sortedResources[j].Value
        })
        return sortedResources[0:5]

}
