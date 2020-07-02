if (document.readyState !== 'loading') {
    internetRadioInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', internetRadioInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') internetRadioInit();
    });
}

function internetRadioInit() {
    if (btn = document.getElementById('internet-radio-database-update')) {
        btn.addEventListener('click', function(event) {
            event.stopPropagation();
            if (window.confirm(dbUpdateConfirm)) {
                var xhr = new XMLHttpRequest();
                xhr.open('GET', '/internet_radio_update');
                xhr.onload = function() {
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
    }
    if (btn = document.getElementById('internet-radio-selected-url-delete-button')) {
        btn.addEventListener('submit', function(event) {
            if (!window.confirm(dbUpdateConfirm)) {
                return false;
            }
            if (elem = document.getElementById('internet-radio-selected-url-delete')) {
                elem.value = '1';
            }
            return true;
        });
    }
    if (frm = document.getElementById('internet-radio-search-form')) {
        frm.addEventListener('submit', function(event) {
            event.preventDefault();
            var params = [];
            for (var i = 0; i < frm.elements.length; i++) {
                if (!frm.elements[i].name) {
                    continue;
                }
                if (frm.elements[i].type == "checkbox") {
                    if (frm.elements[i].checked) {
                        params.push(encodeURIComponent(frm.elements[i].name) + '=' + encodeURIComponent(frm.elements[i].value));
                    }
                } else {
                    params.push(encodeURIComponent(frm.elements[i].name) + '=' + encodeURIComponent(frm.elements[i].value));
                }
            }
            var xhr = new XMLHttpRequest();
            xhr.open(frm.method, frm.action, true);
            xhr.onload = function() {
                var err = '';
                if (xhr.status === 200) {
                    try {
                        treeViewData = JSON.parse(xhr.response);
                        tv = document.getElementById('tree-view');
                        tv.innerHTML = '';
                        addOrphans('#tree-view');
                    } catch (e) {
                        err = e.message;
                    }
                }
                else {
                    err = requestError + xhr.status + ' - ' + xhr.statusText;
                }
                if (err != '') {
                    if (elem = document.getElementById('message-error')) {
                        elem.innerHTML = err;
                    }
                }
            };
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.send(params.join( '&' ).replace( /%20/g, '+' ));
            if (elem = frm.querySelector('legend')) {
                elem.click();
            }
            return false;
        }, false);
    }
}

function spanClick() {
    if (elem = document.getElementById('internet-radio-selected-name')) {
        elem.value = this.innerHTML;
        var i = 0;
        var el = this;
        while(true) {
            el = el.parentElement;
            if (!el) {
                break;
            }
            if (el.tagName == 'LI') {
                if (i == 1) {
                    break;
                }
                i++;
            }
        }
        if (el) {
            for (i = 0; i < el.children.length; i++) {
                if (el.children[i].tagName == 'SPAN') {
                    elem.value = el.children[i].innerHTML + ' - ' + elem.value;
                    break;
                }
            }
        }
    }
    if (elem = document.getElementById('internet-radio-selected-url')) {
        elem.value = this.getAttribute('data-file');
    }
    if (frm = document.getElementById('internet-radio-config-form')) {
        if (item = frm.querySelector('fieldset.collapsible')) {
            if (!item.classList.contains('expanded')) {
                if (elem = item.querySelector('legend')) {
                    elem.click();
                    document.body.scrollTop = 0;
                    document.documentElement.scrollTop = 0;
                }
            }
        }
    }
}
