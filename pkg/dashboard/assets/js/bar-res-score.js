$(function (){
    var data = polarisTopScorers.Resources.map((o, i) => ({name: o.Key, y: o.Value}))
    console.log(polarisTopScorers)

    Highcharts.chart('res-score', {
        chart: {
            type: 'bar'
        },
        title: {
            text: 'Top 5 resources by score'
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
                name: "Scores",
                data: data
            }
        ]
    });
});
