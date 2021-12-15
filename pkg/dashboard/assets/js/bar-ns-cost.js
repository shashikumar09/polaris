$(function (){
    var data = polarisTopWastageCost.Namespaces.map((o, i) => ({name: o.Key, y: o.Value}))
    console.log(data)

    Highcharts.chart('ns-cost', {
        chart: {
            type: 'bar'
        },
        title: {
            text: 'Top 5 namespaces by wastage cost'
        },
        accessibility: {
            announceNewData: {
                enabled: true
            }
        },
        xAxis: {
            type: 'Namespaces'
        },
        yAxis: {
            title: {
                text: 'Score'
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
