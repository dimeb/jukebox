if (document.readyState !== 'loading') {
    listsInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', listsInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') listsInit();
    });
}

function listsInit() {
    document.addEventListener('mouseover', function(e) {
        elem = e.target;
        if (elem.hasAttribute('data-folder')) {
            isFolder = elem.getAttribute('data-folder');
            if (isFolder == '1') {
                elem.style.cursor = (folderSongView || folderView) ? 'pointer' : 'default';
            } else {
                elem.style.cursor = folderView ? 'default' : 'pointer';
            }
        }
    });
    if (frm = document.getElementById('song-search-form')) {
        frm.addEventListener('submit', function(event) {
            event.preventDefault();
            var val = document.getElementById('song-search-input').value;
            data = treeViewData.filter(function(item) {
                if (item.folder == '1') {
                    return false
                }
                var re = new RegExp(val, 'gi');
                if (item.name.match(re)) {
                    return true;
                }
                return false;
            });
            if ((data.length > 0) && (dst = document.getElementById('song-search-result-div'))) {
                for (i = 0; i < data.length; i++) {
                    div = document.createElement('div');
                    div.textContent = data[i].name;
                    if (data[i].file != '.') {
                        div.textContent = data[i].file + '/' + div.textContent;
                    }
                    div.style.cursor = 'pointer';
                    div.style.marginTop = '6px';
                    div.style.marginBottom = '6px';
                    // div.setAttribute('data-file', data[i].file);
                    div.addEventListener('click', spanClick);
                    dst.appendChild(div);
                }
            }
            return false;
        }, false);
    }
    document.querySelectorAll('label.label-content-label').forEach(function(item) {
        if (labelFormat = item.getAttribute('data-format')) {
            a = labelFormat.split('-');
            if (a.length == 4) {
                if (first = item.getElementsByTagName('span')[0]) {
                    first.style.textAlign = a[1];
                }
                if (second = item.getElementsByTagName('span')[1]) {
                    second.style.textAlign = a[3];
                }
            }
        }
    });
    loadLists();
}

function clearSearchSong() {
    document.getElementById('song-search-input').value = '';
    document.getElementById('song-search-result-div').innerHTML = '';
}

function loadLists() {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/lists_search');
    xhr.onload = function() {
        var err = '';
        if (xhr.status === 200) {
            try {
                treeViewData = JSON.parse(xhr.response);
                tv = document.getElementById('tree-view');
                tv.innerHTML = '';
                addOrphans('#tree-view');
                if ((elem = document.getElementById('song-search-div')) && (refElem = document.querySelector('div#tree-view ul'))) {
                    document.getElementById('tree-view').insertBefore(elem, refElem);
                }
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
    xhr.send();
}

function addFolderSong(elem) {
    if (div = document.getElementById('song-search-div')) {
        div.style.display = 'block';
    }
    folderSongView = true;
    folderView = false;
    clearSearchSong();
    toggleModal(elem);
}

function deleteFolders(id) {
    document.querySelectorAll('#' + id + ' input[type="checkbox"]').forEach(function(item) {
        if (item.checked) {
            item.parentElement.remove();
        }
    });
}

function addFolder(elem) {
    if (div = document.getElementById('song-search-div')) {
        div.style.display = 'none';
    }
    folderSongView = false;
    folderView = true;
    clearSearchSong();
    toggleModal(elem);
}

function setSong(elem, slot, song) {
    if (div = document.getElementById('song-search-div')) {
        div.style.display = 'block';
    }
    folderSongView = false;
    folderView = false;
    playSlot = slot;
    playSong = song;
    clearSearchSong();
    toggleModal(elem);
}

function deleteSong(slot, song) {
    const a = ['file', 'name', 'author'];

    a.forEach(function(item, index) {
        if (elem = document.getElementById('play-list-'+item+'-'+slot+'-'+song)) {
            elem.value = '';
        }
    });
    if (elem = document.getElementById('play-list-div-'+slot+'-'+song)) {
        elem.innerHTML = '';
    }
}

function submitLists() {
    document.querySelectorAll('fieldset.collapsible').forEach(function(item) {
        item.querySelector('legend').click();
    });
    checkAll('random_list',true)
    checkAll('browse_list',true)
    return true;
}

function spanClick() {
    var file = this.innerHTML;
    var p = this.getAttribute('data-file');
    if (p != '.') {
        file = p + '/' + file;
    }
    let isFolder = '0';
    if (this.hasAttribute('data-folder')) {
        isFolder = this.getAttribute('data-folder');
    }
    var elem = null;
    if (folderView) {
        if (isFolder == '1') {
            if (elem = document.getElementById('browse-list')) {
                var d = document.createElement('div');
                var cb = document.createElement('input');
                cb.type = 'checkbox';
                cb.name = 'browse_list';
                cb.value = file;
                d.appendChild(cb);
                d.innerHTML += '&nbsp;'+file;
                elem.appendChild(d);
            }
        } else {
            window.alert(onlyFolderSelected);
        }
    } else if (folderSongView) {
        if (elem = document.getElementById('random-list')) {
            var d = document.createElement('div');
            var cb = document.createElement('input');
            cb.type = 'checkbox';
            cb.name = 'random_list';
            cb.value = file;
            d.appendChild(cb);
            d.innerHTML += '&nbsp;'+file;
            elem.appendChild(d);
        }
    } else {
        if (elem = document.getElementById('play-list-file-'+playSlot+'-'+playSong)) {
            elem.value = file;
        }
        var ar = file.split('/');
        const a = ['name', 'author'];
        a.forEach(function(item, index) {
            if (elem = document.getElementById('play-list-'+item+'-'+playSlot+'-'+playSong)) {
                elem.value = ar[ar.length-1];
            }
        });
        if (elem = document.getElementById('play-list-div-'+playSlot+'-'+playSong)) {
            elem.innerHTML = file;
        }
    }
    if (elem = document.getElementById('modal-container-close')) {
        elem.click();
        document.body.scrollTop = document.documentElement.scrollTop = 0;
    }
}
