// 思维树可视化引擎

const treeTranslate = (key, fallback) => {
    if (window.i18n && typeof window.i18n.t === 'function') {
        return window.i18n.t(key, fallback);
    }
    return fallback ?? key;
};

class ThoughtTree {
    constructor(containerId, options = {}) {
        this.container = document.getElementById(containerId);
        this.nodes = new Map();
        this.rootId = null;
        this.handlers = {
            onEdit: typeof options.onEdit === 'function' ? options.onEdit : null,
            onDelete: typeof options.onDelete === 'function' ? options.onDelete : null,
        };
    }

    setHandlers(handlers = {}) {
        if (typeof handlers.onEdit === 'function') {
            this.handlers.onEdit = handlers.onEdit;
        }
        if (typeof handlers.onDelete === 'function') {
            this.handlers.onDelete = handlers.onDelete;
        }
    }

    render(sessionData) {
        if (!this.container) return;
        this.container.innerHTML = '';
        this.nodes.clear();

        if (!sessionData || !sessionData.rootThought) {
            const placeholder = document.createElement('p');
            placeholder.style.opacity = '0.6';
            placeholder.textContent = treeTranslate(
                'treePlaceholder',
                '尚未创建思维树，先输入概念生成新会话。',
            );
            this.container.appendChild(placeholder);
            this.rootId = null;
            return;
        }

        this.rootId = sessionData.rootThought.id;
        const treeList = document.createElement('ul');
        treeList.classList.add('thought-tree');
        this.container.appendChild(treeList);

        const rootNode = new ThoughtNode(sessionData.rootThought, this);
        treeList.appendChild(rootNode.render());
    }

    addNode(thought) {
        if (!thought || !thought.id) return;
        const parentId = thought.parentId ?? (thought.parentID ?? null);
        const parentNode = parentId ? this.nodes.get(parentId) : null;
        const node = new ThoughtNode(thought, this);

        if (parentNode) {
            parentNode.addChild(node);
        } else if (this.container) {
            const rootList = this.container.querySelector('ul.thought-tree');
            if (rootList) rootList.appendChild(node.render());
        }
    }

    updateNode(nodeId, data) {
        const node = this.nodes.get(nodeId);
        if (!node) return;
        node.update(data);
    }

    removeNode(nodeId) {
        const node = this.nodes.get(nodeId);
        if (!node) return;
        node.remove();
        this.nodes.delete(nodeId);
    }

    expandNode(nodeId) {
        const node = this.nodes.get(nodeId);
        if (node) node.expand();
    }

    collapseNode(nodeId) {
        const node = this.nodes.get(nodeId);
        if (node) node.collapse();
    }

    highlightPath(nodeId) {
        for (const node of this.nodes.values()) {
            node.setHighlight(false);
        }
        const node = this.nodes.get(nodeId);
        if (!node || !node.thought.path) return;

        const pathIds = node.thought.pathIds || this.resolvePathIds(node.thought.path);
        pathIds.forEach((id) => {
            const target = this.nodes.get(id);
            if (target) target.setHighlight(true);
        });
    }

    resolvePathIds(path) {
        if (!Array.isArray(path)) return [];
        const ids = [];
        for (const node of this.nodes.values()) {
            if (!node.thought || !Array.isArray(node.thought.path)) continue;
            if (arraysEqual(node.thought.path, path.slice(0, node.thought.path.length))) {
                ids.push(node.thought.id);
            }
        }
        return ids;
    }

    exportToJSON() {
        const tree = [];
        this.nodes.forEach((node) => {
            tree.push(node.toJSON());
        });
        return JSON.stringify(tree, null, 2);
    }

    importFromJSON(data) {
        try {
            const parsed = typeof data === 'string' ? JSON.parse(data) : data;
            this.render({ rootThought: parsed.find((item) => !item.parentId) });
        } catch (error) {
            console.error('Failed to import thought tree:', error);
        }
    }
}

class ThoughtNode {
    constructor(thought, tree) {
        this.thought = thought;
        this.tree = tree;
        this.children = [];
        this.expanded = true;
        this.element = document.createElement('li');
        this.container = document.createElement('div');
        this.container.classList.add('thought-node');
        this.childrenList = document.createElement('ul');
        this.childrenList.classList.add('thought-tree');

        this.renderContent();
        this.element.appendChild(this.container);
        this.element.appendChild(this.childrenList);
        if (tree) tree.nodes.set(thought.id, this);
    }

    render() {
        if (Array.isArray(this.thought.children)) {
            this.thought.children.forEach((child) => {
                const childNode = new ThoughtNode(child, this.tree);
                this.addChild(childNode);
            });
        }
        return this.element;
    }

    renderContent() {
        const title = document.createElement('div');
        title.style.fontWeight = '600';
        title.style.marginBottom = '6px';
    title.textContent = this.thought.content || treeTranslate('unnamedNode', '未命名节点');

        const meta = document.createElement('div');
        meta.style.fontSize = '12px';
        meta.style.opacity = '0.65';
        const directionTitle =
            this.thought.direction?.title ||
            this.thought.direction?.type ||
            treeTranslate('pathFallback', '路径');
        const depthLabel = treeTranslate('depthLabel', '深度');
        meta.textContent = `${directionTitle} · ${depthLabel} ${this.thought.depth ?? 0}`;

        const controls = document.createElement('div');
        controls.style.display = 'flex';
        controls.style.gap = '8px';
        controls.style.marginTop = '10px';

        const toggleBtn = document.createElement('button');
        toggleBtn.classList.add('secondary');
        const collapseLabel = () => treeTranslate('collapse', '折叠');
        const expandLabel = () => treeTranslate('expand', '展开');
        toggleBtn.textContent = collapseLabel();
        toggleBtn.addEventListener('click', () => {
            if (this.expanded) {
                this.collapse();
                toggleBtn.textContent = expandLabel();
            } else {
                this.expand();
                toggleBtn.textContent = collapseLabel();
            }
        });

        controls.appendChild(toggleBtn);

        if (this.tree?.handlers?.onEdit) {
            const editBtn = document.createElement('button');
            editBtn.classList.add('secondary');
            editBtn.textContent = treeTranslate('editNode', '编辑');
            editBtn.addEventListener('click', (event) => {
                event.stopPropagation();
                this.tree.handlers.onEdit?.(this.thought);
            });
            controls.appendChild(editBtn);
        }

        if (this.tree?.handlers?.onDelete) {
            const deleteBtn = document.createElement('button');
            deleteBtn.classList.add('secondary');
            deleteBtn.textContent = treeTranslate('deleteNode', '删除');
            deleteBtn.addEventListener('click', (event) => {
                event.stopPropagation();
                this.tree.handlers.onDelete?.(this.thought);
            });
            controls.appendChild(deleteBtn);
        }

        this.container.innerHTML = '';
        this.container.appendChild(title);
        this.container.appendChild(meta);
        this.container.appendChild(controls);
    }

    update(data) {
        if (!data) return;
        this.thought = { ...this.thought, ...data };
        this.renderContent();
    }

    addChild(childNode) {
        if (!childNode) return;
        this.children.push(childNode);
        this.childrenList.appendChild(childNode.render());
    }

    removeChild(childId) {
        this.children = this.children.filter((child) => {
            if (child.thought.id === childId) {
                child.remove();
                return false;
            }
            return true;
        });
    }

    expand() {
        this.expanded = true;
        this.childrenList.style.display = '';
    }

    collapse() {
        this.expanded = false;
        this.childrenList.style.display = 'none';
    }

    highlight() {
        this.setHighlight(true);
    }

    setHighlight(enabled) {
        if (enabled) {
            this.container.classList.add('highlight');
        } else {
            this.container.classList.remove('highlight');
        }
    }

    remove() {
        this.element.remove();
    }

    toJSON() {
        return { ...this.thought };
    }
}

function arraysEqual(a, b) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    return a.every((value, index) => value === b[index]);
}

// 辅助函数
async function createThoughtVisualization(sessionId, options = {}) {
    const tree = new ThoughtTree('thought-tree', options);
    const session = await loadSessionData(sessionId);
    tree.render(session);
    return tree;
}

async function loadSessionData(sessionId) {
    const response = await fetch(`/api/sessions/${sessionId}`);
    if (!response.ok) throw new Error(await response.text());
    return response.json();
}

function saveSessionData(sessionData) {
    const key = `wideminds-session-${sessionData.id}`;
    localStorage.setItem(key, JSON.stringify(sessionData));
}

window.ThoughtTreeHelpers = {
    createThoughtVisualization,
    loadSessionData,
    saveSessionData,
};
