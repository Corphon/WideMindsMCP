// 交互式画布用于绘制思维导图结构

const canvasTranslate = (key, fallback) => {
    if (window.i18n && typeof window.i18n.t === 'function') {
        return window.i18n.t(key, fallback);
    }
    return fallback ?? key;
};

class InteractiveCanvas {
    constructor(canvasId, options = {}) {
        this.canvas = document.getElementById(canvasId);
        this.options = options;
        this.renderer = this.canvas ? new CanvasRenderer(this.canvas) : null;
        this.scale = 1;
        this.viewport = { x: 0, y: 0 };
        this.nodes = new Map();
        this.session = null;
        this.dragging = false;
        this.dragStart = { x: 0, y: 0 };
    }

    initialize() {
        if (!this.canvas || !this.renderer) return;
        const resizeObserver = new ResizeObserver(() => this.adjustCanvasSize());
        resizeObserver.observe(this.canvas.parentElement || this.canvas);
        this.adjustCanvasSize();
        this.addInteractionHandlers();
        this.resetView();
    }

    adjustCanvasSize() {
        if (!this.canvas) return;
        const rect = this.canvas.getBoundingClientRect();
        const ratio = window.devicePixelRatio || 1;
        this.canvas.width = rect.width * ratio;
        this.canvas.height = rect.height * ratio;
        this.canvas.style.width = `${rect.width}px`;
        this.canvas.style.height = `${rect.height}px`;
        this.renderer?.setPixelRatio(ratio);
        this.render(this.session);
    }

    render(session) {
        this.session = session;
        if (!this.renderer) return;
        this.renderer.clear();
        this.renderer.drawBackground();

        if (!session || !session.rootThought) {
            return;
        }

        this.nodes.clear();
        const layout = this.layoutSession(session);
        this.renderer.setViewport(this.viewport.x, this.viewport.y, this.scale);

            const positionMap = new Map(layout.nodes.map((node) => [node.id, node]));

            layout.edges.forEach((edge) => {
                const fromNode = positionMap.get(edge.from);
                const toNode = positionMap.get(edge.to);
                if (fromNode && toNode) {
                    this.renderer.drawEdge(fromNode, toNode);
                }
            });

            layout.nodes.forEach((node) => {
            this.nodes.set(node.id, node);
                const text = node.label || canvasTranslate('nodeFallback', '节点');
                this.renderer.drawNode({ ...node, label: text }, node.x, node.y);
        });
    }

    layoutSession(session) {
        const nodes = [];
        const edges = [];
        const levels = new Map();

        const queue = [{ thought: session.rootThought, depth: 0 }];
        while (queue.length > 0) {
            const { thought, depth } = queue.shift();
            if (!thought) continue;
            if (!levels.has(depth)) levels.set(depth, []);
            levels.get(depth).push(thought);
            (thought.children || []).forEach((child) => queue.push({ thought: child, depth: depth + 1 }));
        }

        const levelCount = levels.size;
        const canvasWidth = this.canvas.width / (window.devicePixelRatio || 1);
        const levelSpacing = Math.max(140, (this.canvas.height / Math.max(levelCount, 1)) - 20);

        levels.forEach((thoughts, depth) => {
            const spacing = Math.max(180, canvasWidth / Math.max(thoughts.length, 1));
            thoughts.forEach((thought, index) => {
                const node = {
                    id: thought.id,
                    label: thought.content || canvasTranslate('nodeFallback', '节点'),
                    direction: thought.direction,
                    depth: thought.depth,
                    thought,
                    x: spacing / 2 + index * spacing,
                    y: 80 + depth * levelSpacing,
                    radius: 28,
                };
                nodes.push(node);

                if (thought.parentId || thought.parentID) {
                    edges.push({ from: thought.parentId || thought.parentID, to: thought.id });
                }
            });
        });

        return { nodes, edges };
    }

    addInteractionHandlers() {
        if (!this.canvas) return;

        this.canvas.addEventListener('mousedown', (event) => {
            this.dragging = true;
            this.dragStart = { x: event.clientX, y: event.clientY };
        });

        window.addEventListener('mouseup', () => {
            this.dragging = false;
        });

        window.addEventListener('mousemove', (event) => {
            if (!this.dragging) return;
            const dx = event.clientX - this.dragStart.x;
            const dy = event.clientY - this.dragStart.y;
            this.dragStart = { x: event.clientX, y: event.clientY };
            this.onCanvasPan(dx, dy);
        });

        this.canvas.addEventListener('wheel', (event) => {
            event.preventDefault();
            const zoomDelta = event.deltaY < 0 ? 1.1 : 0.9;
            this.onCanvasZoom(this.scale * zoomDelta);
        });

        this.canvas.addEventListener('click', (event) => {
            const node = this.pickNode(event.offsetX, event.offsetY);
            if (node) {
                this.onNodeClick(node.id);
            }
        });

        this.canvas.addEventListener('mousemove', (event) => {
            const node = this.pickNode(event.offsetX, event.offsetY);
            if (node) {
                this.onNodeHover(node.id);
                this.canvas.style.cursor = 'pointer';
            } else {
                this.canvas.style.cursor = 'grab';
            }
        });
    }

    pickNode(offsetX, offsetY) {
        const ratio = window.devicePixelRatio || 1;
        const x = (offsetX * ratio - this.viewport.x) / this.scale;
        const y = (offsetY * ratio - this.viewport.y) / this.scale;
        for (const node of this.nodes.values()) {
            const distance = Math.hypot(node.x - x, node.y - y);
            if (distance <= node.radius + 6) {
                return node;
            }
        }
        return null;
    }

    onNodeClick(nodeId) {
        if (typeof this.options.onNodeClick === 'function') {
            this.options.onNodeClick(nodeId, this.nodes.get(nodeId));
        }
        if (this.options.tree) {
            this.options.tree.highlightPath(nodeId);
        }
    }

    onNodeHover(nodeId) {
        if (typeof this.options.onNodeHover === 'function') {
            this.options.onNodeHover(nodeId, this.nodes.get(nodeId));
        }
    }

    onCanvasZoom(scale) {
        this.scale = Math.min(Math.max(scale, 0.5), 2.5);
        this.render(this.session);
    }

    onCanvasPan(dx, dy) {
        const ratio = window.devicePixelRatio || 1;
        this.viewport.x += dx * ratio;
        this.viewport.y += dy * ratio;
        this.render(this.session);
    }

    resetView() {
        this.scale = 1;
        this.viewport = { x: 0, y: 0 };
        this.render(this.session);
    }

    exportAsImage() {
        if (!this.canvas) return null;
        return this.canvas.toDataURL('image/png');
    }

    clear() {
        this.session = null;
        this.nodes.clear();
        this.renderer?.clear();
    }
}

class CanvasRenderer {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.pixelRatio = window.devicePixelRatio || 1;
    }

    setPixelRatio(ratio) {
        this.pixelRatio = ratio;
    }

    drawNode(node, x, y) {
        if (!this.ctx) return;
        const ctx = this.ctx;
        ctx.save();
        ctx.beginPath();
        const gradient = ctx.createRadialGradient(x - 4, y - 4, node.radius / 4, x, y, node.radius);
        gradient.addColorStop(0, 'rgba(128, 160, 255, 0.9)');
        gradient.addColorStop(1, 'rgba(70, 95, 180, 0.9)');
        ctx.fillStyle = gradient;
        ctx.shadowColor = 'rgba(100, 120, 255, 0.35)';
        ctx.shadowBlur = 16;
        ctx.arc(x, y, node.radius, 0, Math.PI * 2);
        ctx.fill();

        ctx.font = '13px "Segoe UI", sans-serif';
        ctx.fillStyle = 'rgba(255, 255, 255, 0.92)';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
    const text = node.label || canvasTranslate('nodeFallback', '节点');
        wrapText(ctx, text, x, y, node.radius * 1.6, 16);
        ctx.restore();
    }

        drawEdge(fromNode, toNode) {
            if (!this.ctx) return;
            const ctx = this.ctx;
            ctx.save();
            ctx.strokeStyle = 'rgba(126, 170, 255, 0.35)';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(fromNode.x, fromNode.y);
            const controlY = (fromNode.y + toNode.y) / 2;
            ctx.bezierCurveTo(fromNode.x, controlY, toNode.x, controlY, toNode.x, toNode.y);
            ctx.stroke();
            ctx.restore();
    }

    drawBackground() {
        if (!this.ctx) return;
        const ctx = this.ctx;
        ctx.save();
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        const gradient = ctx.createLinearGradient(0, 0, this.canvas.width, this.canvas.height);
        gradient.addColorStop(0, 'rgba(13, 17, 32, 1)');
        gradient.addColorStop(1, 'rgba(18, 24, 44, 1)');
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        ctx.restore();
    }

    clear() {
        if (!this.ctx) return;
        const ctx = this.ctx;
        ctx.save();
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        ctx.restore();
    }

    setViewport(x, y, scale) {
        if (!this.ctx) return;
        this.ctx.setTransform(scale, 0, 0, scale, x, y);
    }
}

function wrapText(ctx, text, x, y, maxWidth, lineHeight) {
    const words = text.split(/\s+/);
    let line = '';
    const lines = [];
    words.forEach((word) => {
        const test = `${line}${word} `;
        if (ctx.measureText(test).width > maxWidth && line !== '') {
            lines.push(line.trim());
            line = `${word} `;
        } else {
            line = test;
        }
    });
    lines.push(line.trim());

    const totalHeight = lines.length * lineHeight;
    let offsetY = y - totalHeight / 2 + lineHeight / 2;
    lines.forEach((current) => {
        ctx.fillText(current, x, offsetY);
        offsetY += lineHeight;
    });
}

function initializeCanvas(canvasId) {
    const canvas = new InteractiveCanvas(canvasId);
    canvas.initialize();
    return canvas;
}

function handleCanvasEvents(canvas, handlers = {}) {
    if (!canvas) return;
    canvas.options = { ...canvas.options, ...handlers };
}

function createNodeElement(thought) {
    const element = document.createElement('div');
    element.classList.add('canvas-node');
    element.textContent = thought.content || canvasTranslate('nodeFallback', '节点');
    return element;
}

function createEdgeElement(from, to) {
    return { from, to };
}

window.CanvasHelpers = {
    initializeCanvas,
    handleCanvasEvents,
    createNodeElement,
    createEdgeElement,
};