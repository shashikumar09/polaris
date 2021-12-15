$(function (){
    var data = polarisTopWastageCost.Resources.map((o, i) => ({name: o.Key, y: o.Value}))
    console.log(polarisTopWastageCost)

    Highcharts.chart('res-cost', {
        chart: {
            type: 'bar'
        },
        title: {
            text: 'Top 5 resources by wastage cost'
        },
        accessibility: {
            announceNewData: {
                enabled: true
            }
        },
        xAxis: {
            type: 'Resources'
        },
        yAxis: {
            title: {
                text: 'Cost'
            }

        },
        legend: {
            enabled: false
        },
        plotOptions: {
            series: {
                borderWidth: 0,
                dataLabels: {
                    enabled: true,
                    format: '{point.y:.1f}%'
                }
            }
        },

        tooltip: {
            headerFormat: '<span style="font-size:11px">{series.name}</span><br>',
            pointFormat: '<span style="color:{point.color}">{point.name}</span>: <b>{point.y:.2f}%</b> of total<br/>'
        },

        series: [
            {
                name: "Cost",
                data: data
            }
        ]
    });
});
