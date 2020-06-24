/* data is in flat array format like this:
[
  {
    id: 'abc',
    name: 'ABCDE',
    parent: null
  },
  {
    id: 'def',
    name: 'DEFGH',
    parent: 'abc'
  }
]
*/

var treeViewData = [];

function orphans() {
  return treeViewData.filter(function(item) {
    return item.parent === null;
  });
}

function hasChildren(parentId) {
  return treeViewData.some(function(item) {
    return item.parent === parentId;
  });
}

function getChildren(parentId) {
  return treeViewData.filter(function(item) {
    return item.parent === parentId;
  });
}

function generateListItem(item) {
  const li = document.createElement('li');
  li.id = 'item-' + item.id;
  if (hasChildren(item.id)) {
    const a = document.createElement('a');
    a.href = '#';
    a.textContent = '+';
    a.classList.add('plus');
    a.addEventListener('click', expand);
    li.appendChild(a);
  }
  const span = document.createElement('span');
  if (typeof(browseContainer) === 'undefined') {
    span.textContent = item.name;
  } else {
    span.textContent = item.name.replace(/\..+$/, '');
    span.setAttribute('data-name', item.name);
  }
  if (item.name == '--' && typeof(unspecified) !== 'undefined') {
    span.textContent = span.textContent + ' - ' + unspecified;
  }
  if (item.file.length > 0) {
    if (typeof(browseContainer) === 'undefined') {
        span.style.cursor = 'pointer';
    }
    span.setAttribute('data-file', item.file);
    if (typeof(spanClick) == 'function') {
      span.addEventListener('click', spanClick);
    }
  }
  if (typeof(item.folder) !== 'undefined') {
    span.setAttribute('data-folder', item.folder);
  }
  li.appendChild(span);
  return li;
}

function expand(event) {
  event.preventDefault();
  event.stopPropagation();
  const et = event.target,
        parent = et.parentElement,
        id = parent.id.replace('item-', ''),
        kids = getChildren(id),
        items = kids.map(generateListItem),
        ul = document.createElement('ul');
  items.forEach(function(li) {
    ul.appendChild(li);
  });
  parent.appendChild(ul);
  et.classList.remove('plus');
  et.classList.add('minus');
  if (typeof(browseContainer) === 'undefined') {
    et.textContent = '-';
  } else {
    et.innerHTML = '&minus;';
  }
  et.removeEventListener('click', expand);
  et.addEventListener('click', collapse);
}

function collapse(event) {
  event.preventDefault();
  event.stopPropagation();
  const et = event.target,
        parent = et.parentElement,
        ul = parent.querySelector('ul');
  parent.removeChild(ul);
  et.classList.remove('minus');
  et.classList.add('plus');
  et.textContent = '+';
  et.removeEventListener('click', collapse);
  et.addEventListener('click', expand);
}

function addOrphans(container) {
  const root = document.querySelector(container),
        orphansArray = orphans();
  if (orphansArray.length) {
    const items = orphansArray.map(generateListItem),
          ul = document.createElement('ul');
    items.forEach(function(li) {
      ul.appendChild(li);
    });
    root.appendChild(ul);
  }
}

// addOrphans();
