if (document.readyState !== 'loading') {
    audioInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', audioInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') audioInit();
    });
}

function audioInit() {
    if (frm = document.getElementById('audio-volume-form')) {
        frm.addEventListener('submit', function(event) {
            event.preventDefault();
            var params = [];
            for (var i = 0; i < frm.elements.length; i++) {
                if (!frm.elements[i].name) {
                    continue;
                }
                params.push(encodeURIComponent(frm.elements[i].name) + '=' + encodeURIComponent(frm.elements[i].value));
            }
            var xhr = new XMLHttpRequest();
            xhr.open(frm.method, frm.action, true);
            xhr.onload = function() {
                if (xhr.status === 200) {
                    if (xhr.response == '') {
                        return;
                    }
                    var arr = xhr.response.split(';');
                    for (var i = 0; i < arr.length; i++) {
                        var kv = arr[i].split('=');
                        if (kv[0] == 'msg') {
                            if (elem = document.getElementById('message-ok')) {
                                elem.innerHTML = kv[1];
                                window.setTimeout(function() {
                                    document.getElementById('message-ok').innerHTML = '';
                                }, 1000);
                            }
                        } else {
                            if (elem = document.querySelector('input[name="'+kv[0]+'"]')) {
                                elem.value = kv[1];
                            }
                        }
                    }
                    document.body.scrollTop = document.documentElement.scrollTop = 0;
                } else {
                    if (e = document.getElementById('message-error')) {
                        e.innerHTML = requestError + xhr.status + ' - ' + xhr.statusText;
                    }
                }
            };
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.send(params.join( '&' ).replace( /%20/g, '+' ));
            if (elem = document.getElementById('audio-volume-value')) {
                elem.value = elem.getAttribute('data-init')
            }
            return false;
        }, false);
    }
}

function setVolume(value) {
    if (elem = document.getElementById('audio-volume-value')) {
        elem.value = value;
        if (btn = document.getElementById('audio-volume-submit-button')) {
            btn.click();
        }
    }
}
