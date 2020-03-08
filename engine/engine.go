package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/leekchan/accounting"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
)

var (
	//TeamSelector is the selector for spliting by team
	TeamSelector = costexplorer.GroupDefinition{
		Key:  aws.String("billed-team"),
		Type: aws.String(costexplorer.GroupDefinitionTypeTag),
	}
	//ServiceSelector is the selector for splitting by services
	ServiceSelector = costexplorer.GroupDefinition{
		Key:  aws.String("billed-service"),
		Type: aws.String(costexplorer.GroupDefinitionTypeTag),
	}
)

//Elements is the set of results
type Elements struct {
	Marker     string
	TimeFrames []string
	Elements   map[string][]float64
	Totals     []float64
}

//NewElements is a constructor for Elements type
func NewElements(frameSize int, marker string) *Elements {
	el := &Elements{}
	el.Marker = marker
	el.TimeFrames = make([]string, frameSize)
	el.Elements = make(map[string][]float64)
	el.Totals = make([]float64, frameSize)
	return el
}

//Extract is the function that extract the selection from aws
func Extract(selector costexplorer.GroupDefinition) *Elements {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	sess.Config.CredentialsChainVerboseErrors = aws.Bool(true)

	srv := costexplorer.New(sess)
	costs, err := srv.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
		Metrics: aws.StringSlice([]string{"BlendedCost"}),
		GroupBy: []*costexplorer.GroupDefinition{
			&selector,
		},
		Granularity: aws.String(costexplorer.GranularityMonthly),
		TimePeriod: &costexplorer.DateInterval{
			End:   aws.String("2020-03-08"),
			Start: aws.String("2019-12-01"),
		},
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			fmt.Println(awsErr.Code(), awsErr.Message())
		}
	}
	marker := "TEAM"
	if selector == ServiceSelector {
		marker = "SERVICE"
	}
	elements := NewElements(len(costs.ResultsByTime), marker)
	for i, v := range costs.ResultsByTime {
		elements.TimeFrames[i] = fmt.Sprintf("%v/%v", *v.TimePeriod.Start, *v.TimePeriod.End)
		for _, elem := range v.Groups {
			unitaryCost, _ := strconv.ParseFloat(*(elem.Metrics["BlendedCost"].Amount), 10)
			for _, k := range elem.Keys {
				splitted := strings.SplitAfter(*k, "$")
				label := "unknown"
				if len(splitted) > 1 && splitted[1] != "" {
					label = splitted[1]
				}
				if _, ok := elements.Elements[label]; !ok {
					elements.Elements[label] = make([]float64, len(costs.ResultsByTime))
				}
				elements.Elements[label][i] = unitaryCost
				elements.Totals[i] += unitaryCost
			}
		}
	}
	return elements

}

//Display is rendering function
func Display(elements *Elements) {
	table := uitable.New()
	table.MaxColWidth = 80
	ac := accounting.Accounting{Symbol: "$", Precision: 2, Thousand: ".", Decimal: ","}
	tmpstring := make([]interface{}, len(elements.TimeFrames)+1)
	tmpstring[0] = color.RedString(elements.Marker)
	for i, v := range elements.TimeFrames {
		tmpstring[i+1] = color.YellowString(v)
	}
	table.AddRow(tmpstring...)
	for k, v := range elements.Elements {
		tmpval := make([]interface{}, len(elements.TimeFrames)+1)
		tmpval[0] = color.GreenString(k)
		for i, j := range v {
			tmpval[i+1] = ac.FormatMoney(j)
		}
		table.AddRow(tmpval...)
	}
	tmpTotal := make([]interface{}, len(elements.TimeFrames)+1)
	tmpTotal[0] = color.HiMagentaString("Tot")
	for i, v := range elements.Totals {
		tmpTotal[i+1] = color.HiMagentaString(ac.FormatMoney(v))
	}
	table.AddRow(tmpTotal...)
	fmt.Println(table)

}
