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

//Extract is the function that extract the selection from aws
func Extract(selector costexplorer.GroupDefinition) {
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
	timeframes := make([]string, 0, len(costs.ResultsByTime))
	teams := make(map[string][]float64)
	totalsPerMonth := make([]float64, len(costs.ResultsByTime))
	for i, v := range costs.ResultsByTime {
		timeframes = append(timeframes, fmt.Sprintf("%v/%v", *v.TimePeriod.Start, *v.TimePeriod.End))
		for _, elem := range v.Groups {
			unitaryCost, _ := strconv.ParseFloat(*(elem.Metrics["BlendedCost"].Amount), 10)
			for _, k := range elem.Keys {
				splitted := strings.SplitAfter(*k, "$")
				label := "unknown"
				if len(splitted) > 1 && splitted[1] != "" {
					label = splitted[1]
				}
				if _, ok := teams[label]; !ok {
					teams[label] = make([]float64, len(costs.ResultsByTime))
				}
				teams[label][i] = unitaryCost
				totalsPerMonth[i] += unitaryCost
			}
		}
	}
	table := uitable.New()
	table.MaxColWidth = 80
	ac := accounting.Accounting{Symbol: "$", Precision: 2, Thousand: ".", Decimal: ","}
	tmpstring := make([]interface{}, len(timeframes)+1)
	tmpstring[0] = color.RedString("TEAMS")
	for i, v := range timeframes {
		tmpstring[i+1] = color.RedString(v)
	}
	table.AddRow(tmpstring...)
	for k, v := range teams {
		tmpval := make([]interface{}, len(timeframes)+1)
		tmpval[0] = color.GreenString(k)
		for i, j := range v {
			tmpval[i+1] = ac.FormatMoney(j)
		}
		table.AddRow(tmpval...)
	}
	tmpTotal := make([]interface{}, len(timeframes)+1)
	tmpTotal[0] = color.HiMagentaString("Tot")
	for i, v := range totalsPerMonth {
		tmpTotal[i+1] = color.HiMagentaString(ac.FormatMoney(v))
	}
	table.AddRow(tmpTotal...)
	fmt.Println(table)

}
