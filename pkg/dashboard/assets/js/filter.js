$(function () {


  // Handle new filter submissions
  $('#namespaceFiltersForm').on('submit', e => {
    e.preventDefault();
    let newParams = new URLSearchParams();
    const resources = new Array($(".js-example-basic-multiple").val());
    const namespaces = new Array($(".js-example-basic-multiple2").val());
    for (let i = 0; i < namespaces.length; i++) {
        newParams.append('ns', namespaces[i]);
    }
    for (let i = 0; i < resources.length; i++) {
        newParams.append('res', resources[i]);
    }
    console.log(newParams)
    console.log(namespaces)
    console.log(resources)
    window.location = new URL(`?${newParams.toString()}`, window.location).toString();
  });
});






