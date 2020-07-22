if (document.readyState !== 'loading') {
    rcloneInit();
} else if (document.addEventListener) {
    document.addEventListener('DOMContentLoaded', rcloneInit);
} else {
    document.attachEvent('onreadystatechange', function() {
        if (document.readyState === 'complete') rcloneInit();
    });
}

function rcloneInit() {
    let xhr = new XMLHttpRequest();
    xhr.open('GET', rcloneURL, true);
    xhr.onload = function() {
        const e = document.getElementById('rclone-iframe-id');
        if (xhr.status === 200) {
            e.innerHTML = xhr.responseText;
        }
    };
    xhr.send();
}

