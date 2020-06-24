if (document.readyState !== 'loading') {
    logsInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', logsInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') logsInit();
    });
}

const ids = ['most-ordered-songs', 'chip-money-inserted']

var mostOrderedSongs = new Litepicker({
    format: 'YYYY-MM-DD',
    numberOfMonths: 1,
    numberOfColumns: 1,
    inlineMode: true,
    singleMode: false,
    autoApply: true,
    showTooltip: false,
    element: document.getElementById('most-ordered-songs')
});
var chipMoneyInserted = new Litepicker({
    format: 'YYYY-MM-DD',
    numberOfMonths: 1,
    numberOfColumns: 1,
    inlineMode: true,
    singleMode: false,
    autoApply: true,
    showTooltip: false,
    element: document.getElementById('chip-money-inserted')
});

function logFileContent(elem) {
    let xhr = new XMLHttpRequest();
    xhr.open(elem.method, elem.action, true);
    xhr.onload = function() {
        const e = document.getElementById('log-file-content-content');
        if (xhr.status === 200) {
            e.value = xhr.responseText;
            e.scrollTop = e.scrollHeight;
        }
        else {
            e.value = logRotateRequestError;
        }
    };
    xhr.send();
}

function logsInit() {
    if (lang == 'en') {
        mostOrderedSongs.lang = 'en_US';
        chipMoneyInserted.lang = 'en_US';
    } else {
        mostOrderedSongs.lang = lang + '_' + lang.toUpperCase();
        chipMoneyInserted.lang = lang + '_' + lang.toUpperCase();
    }
    pickersStyle();
    document.getElementById('logs-logfile-rotate-button').addEventListener('click', function(event) {
        event.stopPropagation();
        if (window.confirm(logRotateConfirm)) {
            let xhr = new XMLHttpRequest();
            xhr.open('GET', '/rotate_log');
            xhr.onload = function() {
                let e = null;
                if (xhr.status === 200) {
                    if (e = document.getElementById('message-ok')) {
                        e.innerHTML = logRotateRequestSent;
                    }
                }
                else {
                    if (e = document.getElementById('message-error')) {
                        e.innerHTML = logRotateRequestError;
                    }
                }
                document.body.scrollTop = document.documentElement.scrollTop = 0;
            };
            xhr.send();
        }
        return;
    });
    let frm = document.getElementById('log-file-content-form');
    if (frm) {
        logFileContent(frm);
        frm.addEventListener('submit', function(event) {
            event.preventDefault();
            logFileContent(frm);
            return false;
        }, false);
    }
    for (var i = 0; i < ids.length; i++) {
        let id = ids[i];
        document.getElementById(id).addEventListener('click', function(event) {
            pickersStyle();
        });
        let frm = document.getElementById(id + '-form');
        if (!frm) {
            continue;
        }
        frm.addEventListener('submit', function(event) {
            event.preventDefault();
            let params = [];
            for (var j = 0; j < frm.elements.length; j++) {
                if (!frm.elements[j].name) {
                    continue;
                }
                params.push(encodeURIComponent(frm.elements[j].name) + '=' + encodeURIComponent(frm.elements[j].value));
            }
            let xhr = new XMLHttpRequest();
            xhr.open(frm.method, frm.action, true);
            xhr.onload = function() {
                const e = document.getElementById(id + '-list');
                if (xhr.status === 200) {
                    e.innerHTML = xhr.responseText;
                }
                else {
                    e.innerHTML = logRotateRequestError;
                }
            };
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.send(params.join( '&' ).replace( /%20/g, '+' ));
            return false;
        }, false);
    }
}

function pickersStyle() {
    document.querySelectorAll('div.litepicker').forEach(function(item) {
        item.style.display = 'block';
    });
}

function mostOrderedSongsClear() {
    document.getElementById('most-ordered-songs').value = '';
    document.getElementById('most-ordered-songs-list').innerHTML = '';
}

function chipMoneyInsertedClear() {
    document.getElementById('chip-money-inserted').value = '';
    document.getElementById('chip-money-inserted-list').innerHTML = '';
}
