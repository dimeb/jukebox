if (document.readyState !== 'loading') {
    appInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', appInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') appInit();
    });
}

const buttonCodes = 'abcdefghijklmnopqrstuvwx';

var screenData = null;
var playData = [];
var playListNumber = '';
var canSelect = 0;
var isMoving = false;
var moveXStart = 0;
var moveYStart = 0;

var webSocket = {
    conn: null,
    open: function() {
        if (this.conn instanceof WebSocket && this.conn.readyState == 1 /* OPEN */) {
            return;
        }
        this.conn = new WebSocket('ws://' + document.location.host + '/data');
        console.log('Web socket opened');
    },
    send: function(message) {
        if (this.conn instanceof WebSocket && this.conn.readyState == 1 /* OPEN */) {
            this.conn.send(JSON.stringify(message));
        }
    },
    close: function(reloadPage = false) {
        this.conn.close();
    }
}

function textFit(elem, width) {
    if (elem.scrollWidth > width) {
        elem.innerHTML = '<span style="font-size:smaller;white-space:normal;">' + elem.innerHTML + '</span>';
    }
}

function setUsage() {
    img = document.getElementById('icon-usage');
    src = '';
    if (screenData.SelectionSource == 'free' || canSelect > 0) {
        document.getElementById('label-usage-text').innerHTML = skinUsageText();
        src = 'usage';
    } else {
        if (screenData.SelectionSource == 'chip') {
            document.getElementById('label-usage-text').innerHTML = screenData.ChipText;
            src = 'chip';
        } else if (screenData.SelectionSource == 'money') {
            document.getElementById('label-usage-text').innerHTML = screenData.MoneyText;
            src = 'money';
        }
    }
    if (img && src !== '') {
        img.src = '/img/'+skin+'/icon_'+src+'.svg';
    }
    skinSetUsage();
}

function coinAlert() {
    const elem = document.getElementById('label-selected-selection');
    elem.style.color = 'red';
    elem.innerHTML = document.getElementById('label-usage-text').innerHTML;
    setTimeout(function() {
        elem.style.color = document.body.style.color;
        elem.innerHTML = '';
    }, 2000, elem);
}

function headerInit() {
    setUsage();
    document.getElementById('label-selected-text').innerHTML = screenData.SongSelectedText;
    document.getElementById('button-cancel-text').innerHTML = screenData.CancelSelectionText;
    document.getElementById('button-play-text').innerHTML = screenData.PlaySelectionText;
}

function bodyInit() {
    if (typeof(browseContainer) === 'undefined') {
        let top = 'name';
        let topAlign = 'left';
        let bottomAlign = 'left';
        const arr = screenData.LabelContent.split('-');
        if (arr.length == 4) {
            top = arr[0];
            topAlign = arr[1];
            bottomAlign = arr[3];
        }
        if (screenData.PlayLists.length > 0) {
            if (playListNumber == '' || !screenData.PlayLists.includes(playListNumber)) {
                playListNumber = screenData.PlayLists[0];
            }
            for (var i = 0; i < buttonCodes.length; i++) {
                const chr = buttonCodes.substring(i, i + 1);
                const divTop = document.getElementById('label-top-'+chr);
                const divBottom = document.getElementById('label-bottom-'+chr);
                divTop.style.textAlign = topAlign;
                divBottom.style.textAlign = bottomAlign;
                divTop.innerHTML = '';
                divBottom.innerHTML = '';
                divTopWidth = divTop.clientWidth;
                divBottomWidth = divBottom.clientWidth;
                if (chr in screenData.Songs[playListNumber]) {
                    const name = screenData.Songs[playListNumber][chr].Name.trim();
                    const author = screenData.Songs[playListNumber][chr].Author.trim();
                    if (top == 'name') {
                        divTop.innerHTML = name;
                        divBottom.innerHTML = author;
                    } else {
                        divTop.innerHTML = author;
                        divBottom.innerHTML = name;
                    }
                }
                textFit(divTop, divTopWidth);
                textFit(divBottom, divBottomWidth);
            }
        }
    }
    skinBodyInit();
}

function footerInit() {
    if (typeof(browseContainer) !== 'undefined') {
        return;
    }
    if (screenData.PlayLists.length < 2) {
        return;
    }
    const table = document.getElementById('play-list-selector')
    table.innerHTML = '';
    var objw = document.createElement('div');
    var obj = document.createElement('div');
    obj.innerHTML = screenData.ListText;
    objw.appendChild(obj);
    obj = document.createElement('img');
    obj.src = '/img/'+skin+'/hand_pointing_right.svg';
    objw.appendChild(obj);
    table.appendChild(objw);
    objw = document.createElement('div');
    obj = document.createElement('img');
    obj.id = 'button-play-list-left';
    obj.src = '/img/'+skin+'/button_left.svg';
    obj.onclick = function(){ playListChange(false); };
    objw.appendChild(obj);
    table.appendChild(objw);
    for (var i = 0; i < screenData.PlayLists.length; i++) {
        const playList = screenData.PlayLists[i];
        objw = document.createElement('div');
        obj = document.createElement('img');
        obj.id = 'button-list-'+i;
        obj.src = '/img/'+skin+'/list'+(playListNumber == playList ? '_selected' : '')+'.svg';
        objw.appendChild(obj);
        table.appendChild(objw);
    }
    objw = document.createElement('div');
    obj = document.createElement('img');
    obj.id = 'button-play-list-right';
    obj.src = '/img/'+skin+'/button_right.svg';
    obj.onclick = function(){ playListChange(true); };
    objw.appendChild(obj);
    table.appendChild(objw);
    objw = document.createElement('div');
    obj = document.createElement('img');
    obj.src = '/img/'+skin+'/hand_pointing_left.svg';
    objw.appendChild(obj);
    obj = document.createElement('div');
    obj.innerHTML = screenData.ListText;
    objw.appendChild(obj);
    table.appendChild(objw);
}

function playListChange(forward = true) {
    const len = screenData.PlayLists.length;
    let pos = screenData.PlayLists.indexOf(playListNumber);
    if (pos < 0) {
        return;
    }
    if (forward) {
        if (++pos >= len) {
            pos = 0;
        }
    } else {
        if (--pos < 0) {
            pos = len - 1;
        }
    }
    playListNumber = screenData.PlayLists[pos];
    bodyInit();
    footerInit();
}

function screenInit() {
    headerInit()
    bodyInit();
    footerInit();
    document.querySelectorAll('img').forEach(function(img) {
        img.setAttribute('draggable', 'false');
    });
}

function songSelection(chr) {
    if (canSelect > 0) {
        if (canSelect <= playData.length) {
            playData.shift();
        }
        playData.push(playListNumber + '-' + chr);
        const elem = document.getElementById('label-selected-selection');
        elem.innerHTML = '';
        const elemWidth = elem.clientWidth;
        let a = [];
        for (var j = 0; j < playData.length; j++) {
            const pln = playData[j].substring(0, 1);
            const plc = playData[j].substring(2, 3);
            if (plc in screenData.Songs[pln]) {
                a.push('"' + screenData.Songs[pln][plc].Name.trim() + '" ' + screenData.Songs[pln][plc].Author.trim());
            }
        }
        elem.innerHTML = a.join('<br>');
        textFit(elem, elemWidth);
    } else {
        coinAlert();
    }
}

function appReload() {
    const xhr = new XMLHttpRequest();
    xhr.addEventListener('load', function() {
        window.location.reload(true);
    });
    xhr.addEventListener('error', function() {
        const elem = document.getElementById('label-selected-selection');
        elem.style.color = 'red';
        elem.innerHTML = screenData.ErrorText;
        setTimeout(function() {
            elem.style.color = document.body.style.color;
            elem.innerHTML = '';
        }, 500, elem);
    });
    xhr.open('GET', window.location.href);
    xhr.setRequestHeader('Pragma','no-cache');
    xhr.setRequestHeader('Expired','-1');
    xhr.setRequestHeader('Cache-Control','no-cache');
    xhr.send();
}

function appInit() {
    skinInit();
    if (typeof(browseContainer) === 'undefined') {
        document.body.addEventListener('mousedown', function (event) {
            isMoving = true;
            moveXStart = event.pageX;
            moveYStart = event.pageY;
        });
        document.body.addEventListener('mouseup', function (event) {
            if (isMoving) {
                const x = event.pageX - moveXStart;
                const y = event.pageY - moveYStart;
                moveXStart = 0;
                moveYStart = 0;
                isMoving = false;
                if (x > 50) {
                    playListChange(false);
                } else if (x < -50) {
                    playListChange();
                } else if (y > 50) {
                    playListChange(false);
                } else if (y < -50) {
                    playListChange();
                }
            }
        });
    }
    // window.addEventListener('resize', function(event) {
    //     document.body.style.cursor = window.innerHeight == window.screen.height ? 'none' : 'default';
    //     bodyInit();
    // });
    document.getElementById('button-cancel-text').addEventListener('click', function(event) {
        if (canSelect > 0) {
            webSocket.send({messageType: 'coin'});
            canSelect = 0;
            playData = [];
            document.getElementById('label-selected-selection').innerHTML = '';
            setUsage();
        } else {
            coinAlert();
        }
    });
    document.getElementById('button-play-text').addEventListener('click', function(event) {
        if (canSelect > 0) {
            let msgType = 'play';
            if (typeof(browseContainer) !== 'undefined') {
                msgType = 'browse_play';
            }
            webSocket.send({
                messageType: msgType,
                messageData: playData.join(',')
            });
            canSelect = 0;
            playData = [];
            document.getElementById('label-selected-selection').innerHTML = '';
            setUsage();
        } else {
            coinAlert();
        }
    });
    webSocket.open();
    webSocket.conn.onclose = function(event) {
        console.log('Web socket closed');
        setInterval(function() {
            appReload();
            //webSocket.open();
        }, 1000);
    };
    webSocket.conn.onerror = function(event) {
        console.log('Web socket error: ' + event.message);
    };
    webSocket.conn.onmessage = function(event) {
        try {
            let data = JSON.parse(event.data);
            let addText = 0;
            switch (data.messageType) {
                case 'browseInit':
                    if (typeof(treeViewData) !== 'undefined') {
                        try {
                            treeViewData = JSON.parse(data.messageData);
                            bodyInit();
                        } catch (e) {
                            console.log('Receive error: ' + e.message);
                        }
                    }
                    break;
                case 'init':
                    try {
                        screenData = JSON.parse(data.messageData);
                        canSelect = screenData.SelectionSource == 'free' ? 1 : 0;
                        screenInit();
                        bodyInit();
                        if (canSelect > 1) {
                            addText = canSelect;
                        }
                        if (typeof(browseContainer) !== 'undefined') {
                            webSocket.send({
                                messageType: 'browseInit',
                                messageData: ''
                            });
                        }
                    } catch (e) {
                        console.log('Receive error: ' + e.message);
                    }
                    break;
                case 'skin':
                    appReload();
                    break;
                case 'coin':
                    canSelect = data.messageData;
                    setUsage();
                    if (canSelect > 1) {
                        addText = canSelect;
                    }
                    break;
                case 'modalImage':
                    const a = data.messageData.split('#');
                    if (a.length == 2) {
                        const elem = document.getElementById(a[0]);
                        if (elem && a[1].length > 0) {
                            elem.src = 'data:image/' + a[1];
                        }
                    }
            }
            if (addText > 0) {
                document.getElementById('label-selected-text').innerHTML += '<br><span style="font-size:smaller;">('+addText+' '+screenData.SongsText+')</span>';
            }
        } catch (e) {
            console.log('Receive error: ' + e.message);
        }
    }
}

