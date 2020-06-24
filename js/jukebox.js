if (document.readyState !== 'loading') {
    pageInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', pageInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') pageInit();
    });
}

function pageInit() {
    if (typeof pageTitle !== 'undefined') {
        document.querySelectorAll('div.pagetitle > h2').forEach(function(item) {
            item.innerHTML = pageTitle;
        });
    }
    document.querySelectorAll('.input-type-file-label').forEach(function(item) {
        item.innerHTML = item.getAttribute('data-text');
    });
    document.querySelectorAll('.input-type-file-button').forEach(function(item) {
        item.onclick = function() {
            var a = item.id.split('-');
            a[a.length - 1] = 'file';
            var elem = document.getElementById(a.join('-'));
            if (elem !== null) {
                elem.onchange = function() {
                    var fNames = new Array();
                    for (var i = 0; i < this.files.length; i++) {
                        if (this.hasAttribute('data-maxsize')) {
                            var x = parseInt(this.getAttribute('data-maxsize'));
                            if (this.files[i].size > x) {
                                window.alert(fileIsGreater);
                                this.value = '';
                            } else {
                                fNames.push(this.files[i].name);
                            }
                        } else {
                            fNames.push(this.files[i].name);
                        }
                    }
                    var a = this.id.split('-');
                    a[a.length - 1] = 'label';
                    var e = document.getElementById(a.join('-'))
                    if (e !== null) {
                        e.innerHTML = (fNames.length > 0) ? fNames.join(', ') : e.getAttribute('data-text');
                    }
                };
                elem.click();
            }
        };
    });
    document.querySelectorAll('fieldset.collapsible').forEach(function(item) {
        var legend = item.querySelector('legend');
        legend.onclick = function() {
            item.classList.toggle('expanded');
            if (elem = legend.querySelector('span')) {
                elem.innerHTML = item.classList.contains('expanded') ? '-' : '+';
            }
        };
    });
}

function checkAll(name, check) {
    document.querySelectorAll('input[name="'+name+'"]').forEach(function(item) {
        item.checked = check;
    });
}

function toggleModal(trigger) {
    if (id = trigger.getAttribute('data-target')) {
        if (target = document.getElementById(id)) {
            classes = target.classList;
            if (classes.contains('modal')) {
                classes.toggle('modal-open');
            }
        }
    }
}
