if (document.readyState !== 'loading') {
    streamingServicesInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', streamingServicesInit);
} else {
    document.attachEvent('onreadystatechange', function () {
        if (document.readyState === 'complete') streamingServicesInit();
    });
}

function streamingServicesInit() {
    document.querySelectorAll('div.streaming-services-update').forEach(function (item) {
        item.addEventListener('click', function (event) {
            event.stopPropagation();
            if (window.confirm(dbUpdateConfirm)) {
                var xhr = new XMLHttpRequest();
                xhr.open('GET', '/streaming_services_update?origin=' + encodeURIComponent(item.getAttribute('data-origin')));
                xhr.onload = function () {
                    var e = null;
                    if (xhr.status === 200) {
                        if (e = document.getElementById('message-ok')) {
                            e.innerHTML = dbUpdateRequestSent
                        }
                    }
                    else {
                        if (e = document.getElementById('message-error')) {
                            e.innerHTML = dbUpdateRequestError
                        }
                    }
                    document.body.scrollTop = document.documentElement.scrollTop = 0;
                };
                xhr.send();
            }
            return;
        });
    });
}
