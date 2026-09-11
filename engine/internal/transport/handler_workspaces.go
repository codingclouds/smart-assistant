package transport

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/smart-assistant/engine/internal/workspace"
)

// ==================== 工作空间处理 ====================

// handleWorkspacesList 列出当前用户的工作空间。
func handleWorkspacesList(c *Conn, id string, params []byte) {
	workspaces, err := c.router.WorkspaceSvc.GetByUser(c.UserID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, workspaces)
}

// handleWorkspacesCreate 创建新的工作空间。
func handleWorkspacesCreate(c *Conn, id string, params []byte) {
	var req struct {
		Name string `json:"name"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.Name == "" {
		c.sendError(id, -32001, "name is required")
		return
	}

	ws, err := c.router.WorkspaceSvc.Create(req.Name, c.UserID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, ws)
}

// handleWorkspaceFiles 列出工作空间中的文件。
func handleWorkspaceFiles(c *Conn, id string, params []byte) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.WorkspaceID == "" {
		c.sendError(id, -32001, "workspace_id is required")
		return
	}

	filesDir := workspace.FilesDir(req.WorkspaceID)
	var files []map[string]interface{}

	err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(filesDir, path)
		files = append(files, map[string]interface{}{
			"path":        relPath,
			"size":        info.Size(),
			"modified_at": info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
		return nil
	})

	if err != nil || files == nil {
		files = []map[string]interface{}{}
	}

	c.sendResult(id, files)
}

// ==================== 工作空间文件内容/预览（通过 WebSocket RPC，无需 HTTP fetch） ====================

// handleWorkspaceFileContent 获取工作空间文件内容用于文本/图片预览。
func handleWorkspaceFileContent(c *Conn, id string, params []byte) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		FilePath    string `json:"file_path"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.WorkspaceID == "" || req.FilePath == "" {
		c.sendError(id, -32001, "workspace_id and file_path are required")
		return
	}

	fullPath, err := resolveWorkspaceFilePath(req.WorkspaceID, req.FilePath)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.sendError(id, -32002, "file not found")
		} else {
			c.sendError(id, -32001, "failed to read file: "+err.Error())
		}
		return
	}

	mimeType := detectMimeType(req.FilePath)
	resp := map[string]interface{}{
		"file_name":    filepath.Base(req.FilePath),
		"file_path":    req.FilePath,
		"content_type": mimeType,
		"size":         len(data),
		"workspace_id": req.WorkspaceID,
	}

	if isTextMime(mimeType) {
		resp["content"] = string(data)
	} else {
		// 二进制文件（图片等）：base64 编码后在 JSON 中传输
		resp["content"] = base64.StdEncoding.EncodeToString(data)
	}

	c.sendResult(id, resp)
}

// handleWorkspaceFilePreview 将 Office 文档转换为 HTML 用于预览。
func handleWorkspaceFilePreview(c *Conn, id string, params []byte) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		FilePath    string `json:"file_path"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.WorkspaceID == "" || req.FilePath == "" {
		c.sendError(id, -32001, "workspace_id and file_path are required")
		return
	}

	log.Printf("[preview] request workspace=%s file=%s", req.WorkspaceID, req.FilePath)

	fullPath, err := resolveWorkspaceFilePath(req.WorkspaceID, req.FilePath)
	if err != nil {
		log.Printf("[preview] resolve error: %v", err)
		c.sendError(id, -32001, err.Error())
		return
	}

	log.Printf("[preview] fullPath=%s", fullPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[preview] file not found: %s", fullPath)
			c.sendError(id, -32002, "file not found")
		} else {
			log.Printf("[preview] stat error: %v", err)
			c.sendError(id, -32001, "failed to stat file: "+err.Error())
		}
		return
	}

	ext := strings.ToLower(filepath.Ext(req.FilePath))
	log.Printf("[preview] ext=%s, size=%d", ext, info.Size())

	resp := map[string]interface{}{
		"file_name":         filepath.Base(req.FilePath),
		"file_path":         req.FilePath,
		"content_type":      detectMimeType(req.FilePath),
		"size":              info.Size(),
		"workspace_id":      req.WorkspaceID,
		"preview_available": false,
		"preview_html":      "",
		"preview_error":     "",
	}

	if !isOfficeDocument(req.FilePath) {
		resp["preview_error"] = fmt.Sprintf("preview not supported for %s files", ext)
		c.sendResult(id, resp)
		return
	}

	log.Printf("[preview] calling convertOfficeToHTML...")
	html, convErr := convertOfficeToHTML(fullPath, ext)
	if convErr != nil {
		log.Printf("[preview] conversion error: %v", convErr)
		resp["preview_error"] = convErr.Error()
	} else {
		log.Printf("[preview] conversion success, html=%d bytes", len(html))
		resp["preview_available"] = true
		resp["preview_html"] = html
	}

	c.sendResult(id, resp)
	log.Printf("[preview] response sent for id=%s", id)
}

// resolveWorkspaceFilePath 解析并验证工作空间文件路径。
func resolveWorkspaceFilePath(workspaceID, filePath string) (string, error) {
	workspaceRoot := workspace.FilesDir(workspaceID)
	cleanPath := filepath.Clean(filePath)

	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	fullPath := filepath.Join(workspaceRoot, cleanPath)
	fullPath = filepath.Clean(fullPath)

	sep := string(filepath.Separator)
	if !strings.HasPrefix(fullPath, workspaceRoot+sep) && fullPath != workspaceRoot {
		return "", fmt.Errorf("path traversal detected")
	}

	return fullPath, nil
}

// detectMimeType 根据文件扩展名推断 MIME 类型。
func detectMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".ts", ".tsx":
		return "text/typescript"
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt", ".log":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".toml":
		return "application/toml"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java"
	case ".rs":
		return "text/x-rust"
	case ".sh", ".bash":
		return "text/x-shellscript"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

// isTextMime 判断 MIME 类型是否为文本。
func isTextMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		mimeType == "application/javascript" ||
		mimeType == "application/yaml" ||
		mimeType == "application/toml"
}

// isOfficeDocument 判断文件是否为可转换的 Office 文档格式。
func isOfficeDocument(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp":
		return true
	}
	return false
}

// convertOfficeToHTML 使用 Python 将 Office 文档转换为 HTML。
func convertOfficeToHTML(filePath, ext string) (string, error) {
	script := officeToHTMLScript(filePath, ext)
	log.Printf("[preview] converting %s (%s), script length=%d bytes", filePath, ext, len(script))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	log.Printf("[preview] python3 finished in %v, stdout=%d bytes, stderr=%d bytes, err=%v", elapsed, stdout.Len(), stderr.Len(), err)

	if err != nil {
		return "", fmt.Errorf("python3 conversion failed: %v — stderr: %s", err, stderr.String())
	}

	result := stdout.String()
	if result == "" {
		return "", fmt.Errorf("python3 produced empty output — stderr: %s", stderr.String())
	}
	return result, nil
}

// officeToHTMLScript 根据文件扩展名返回对应的 Python 转换脚本。
func officeToHTMLScript(filePath, ext string) string {
	switch ext {
	case ".docx":
		return docxToHTMLScript(filePath)
	case ".xlsx":
		return xlsxToHTMLScript(filePath)
	case ".pptx", ".odp":
		return pptxToHTMLScript(filePath)
	case ".odt":
		return odtToHTMLScript(filePath)
	case ".ods":
		return odsToHTMLScript(filePath)
	default:
		return fmt.Sprintf(`print('<p>unsupported format: %s</p>')`, ext)
	}
}

// docxToHTMLScript 生成将 .docx 转换为 HTML 的 Python 脚本。
// 使用 python-docx 解析富文本格式（加粗、斜体、下划线、颜色），
// 渲染表格（首行自动识别为表头）、列表、图片，回退到 zipfile+XML。
func docxToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, os, base64, html as html_mod
filepath = %q

CSS = '''<style>
.docx-body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#e4e5e9;line-height:1.85;font-size:14px;padding:8px 0}
.docx-body h1{font-size:1.55rem;font-weight:700;margin:24px 0 12px;padding-bottom:8px;border-bottom:1px solid rgba(255,255,255,0.1);color:#f0f1f3}
.docx-body h2{font-size:1.3rem;font-weight:600;margin:20px 0 10px;color:#ebecee}
.docx-body h3{font-size:1.14rem;font-weight:600;margin:16px 0 8px;color:#e4e5e9}
.docx-body h4{font-size:1.04rem;font-weight:600;margin:12px 0 6px;color:#d4d5d9}
.docx-body p{margin:6px 0}
.docx-body table{width:100%%;border-collapse:collapse;margin:14px 0;font-size:13px;border:1px solid rgba(255,255,255,0.1);border-radius:8px;overflow:hidden}
.docx-body th{background:rgba(255,255,255,0.08);color:#ebecee;font-weight:600;padding:10px 14px;text-align:left;border-bottom:1px solid rgba(255,255,255,0.14);font-size:12px;letter-spacing:.02em}
.docx-body td{padding:9px 14px;border-bottom:1px solid rgba(255,255,255,0.06);color:#cfd1d6;vertical-align:top}
.docx-body tr:last-child td{border-bottom:none}
.docx-body tbody tr:hover{background:rgba(255,255,255,0.025)}
.docx-body ul,.docx-body ol{padding-left:2em;margin:8px 0}
.docx-body li{margin-bottom:4px}
.docx-body img{max-width:100%%;height:auto;border-radius:6px;margin:10px 0}
.docx-body blockquote{border-left:3px solid #818cf8;padding:6px 16px;margin:12px 0;background:rgba(129,140,248,0.06);border-radius:0 6px 6px 0;color:#b0b3be}
.docx-body code{background:rgba(255,255,255,0.08);padding:2px 6px;border-radius:4px;font-size:.89em;font-family:"SF Mono","Fira Code",Menlo,monospace;color:#f87171}
.docx-body pre{background:rgba(0,0,0,0.22);padding:14px 18px;border-radius:8px;overflow-x:auto;margin:10px 0;border:1px solid rgba(255,255,255,0.08);font-size:13px}
.docx-body pre code{background:none;padding:0;color:#d4d5d9}
.docx-body hr{border:none;border-top:1px solid rgba(255,255,255,0.08);margin:20px 0}
</style>'''

# ========== 方案 1: python-docx（富文本 + 表格 + 列表 + 图片）==========
try:
    from docx import Document
    from docx.shared import Inches, Pt, RGBColor
    from docx.oxml.ns import qn

    doc = Document(filepath)
    out = [CSS, '<div class="docx-body">']

    ALIGN = {0:'left',1:'center',2:'right',3:'justify'}
    W_NS = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'
    WP_NS = 'http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing'
    A_NS  = 'http://schemas.openxmlformats.org/drawingml/2006/main'
    R_NS  = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'

    def get_images():
        """从 document.xml 建立 rId -> image part 的映射。"""
        imap = {}
        try:
            body = doc.element.body
            for blip in body.iter('{%%s}blip' %% A_NS):
                embed = blip.get('{%%s}embed' %% R_NS)
                if embed:
                    try:
                        imap[embed] = doc.part.related_parts[embed]
                    except:
                        pass
        except:
            pass
        return imap

    img_map = get_images()

    def img_tag(embed_id):
        if embed_id not in img_map:
            return ''
        part = img_map[embed_id]
        b64 = base64.b64encode(part.blob).decode()
        ct = getattr(part, 'content_type', 'image/png') or 'image/png'
        ext = ct.split('/')[-1]
        if ext == 'jpeg': ext = 'jpg'
        return '<img src="data:image/%%s;base64,%%s" alt="" loading="lazy">' %% (ext, b64)

    def render_run(run):
        t = html_mod.escape(run.text or '')
        if not t: return ''
        if run.bold:      t = '<strong>%%s</strong>' %% t
        if run.italic:    t = '<em>%%s</em>' %% t
        if run.underline: t = '<u>%%s</u>' %% t
        if run.font.color and run.font.color.rgb:
            t = '<span style="color:#%%s">%%s</span>' %% (str(run.font.color.rgb), t)
        if run.font.size:
            t = '<span style="font-size:%%.1fpx">%%s</span>' %% (run.font.size.pt, t)
        return t

    def extract_images(para):
        """从段落 XML 中提取内嵌图片。"""
        buf = []
        for r in para._element.iter('{%%s}drawing' %% W_NS):
            for blip in r.iter('{%%s}blip' %% A_NS):
                e = blip.get('{%%s}embed' %% R_NS)
                if e:
                    buf.append(img_tag(e))
        for r in para._element.iter('{%%s}pict' %% W_NS):
            for shape in r.iter('{%%s}imagedata' %% 'urn:schemas-microsoft-com:vml'):
                rid = shape.get('{%%s}id' %% R_NS)
                if rid:
                    buf.append(img_tag(rid))
        return ''.join(buf)

    def para_align(para):
        a = para.alignment
        return 'text-align:%%s;' %% ALIGN.get(int(a), 'left') if a is not None else ''

    # ---------- 检查列表 ----------
    # 预扫描 numbering 信息
    numbering = doc.part.numbering_part
    num_id_map = {}  # numId -> (abstractNumId, 是否为有序)
    if numbering:
        try:
            nx = numbering._element
            for num in nx.findall('{%%s}num' %% W_NS):
                nid = num.get('{%%s}numId' %% W_NS)
                ab = num.find('{%%s}abstractNumId' %% W_NS)
                abid = ab.get('{%%s}val' %% W_NS) if ab is not None else None
                num_id_map[nid] = (abid, False)
            # 判断 abstractNum 是否为有序
            for anum in nx.findall('{%%s}abstractNum' %% W_NS):
                aid = anum.get('{%%s}abstractNumId' %% W_NS)
                for lvl in anum.findall('{%%s}lvl' %% W_NS):
                    nf = lvl.find('{%%s}numFmt' %% W_NS)
                    is_ordered = False
                    if nf is not None:
                        fmt = nf.get('{%%s}val' %% W_NS, '')
                        is_ordered = fmt not in ('bullet',)
                    # 更新所有引用此 abstractNum 的 numId
                    for nid, (abid, _) in list(num_id_map.items()):
                        if abid == aid:
                            num_id_map[nid] = (abid, is_ordered)
        except:
            pass

    def get_list_info(p_elem):
        """返回 (numId, ilvl) 或 (None, None)。"""
        numPr = p_elem.find('{%%s}pPr/{%%s}numPr' %% (W_NS, W_NS))
        if numPr is None:
            return None, None
        nid_el = numPr.find('{%%s}numId' %% W_NS)
        ilvl_el = numPr.find('{%%s}ilvl' %% W_NS)
        nid = nid_el.get('{%%s}val' %% W_NS) if nid_el is not None else None
        ilvl = int(ilvl_el.get('{%%s}val' %% W_NS, '0')) if ilvl_el is not None else 0
        return nid, ilvl

    # ---------- 遍历 document body ----------
    body = doc.element.body
    paragraphs = doc.paragraphs  # 缓存
    para_index = -1

    open_list = None   # None | ('ol'|'ul', list_level)

    def close_list():
        global open_list
        if open_list:
            tag = open_list[0]
            out.append('</%%s>' %% tag)
            open_list = None

    for child in body:
        tag = child.tag.split('}')[-1] if '}' in child.tag else child.tag

        if tag == 'p':
            para_index += 1
            if para_index >= len(paragraphs):
                continue
            para = paragraphs[para_index]
            align_style = para_align(para)

            # 检测列表
            nid, ilvl = get_list_info(child)
            if nid is not None:
                _, is_ordered = num_id_map.get(nid, (None, False))
                list_tag = 'ol' if is_ordered else 'ul'
                if open_list is None or open_list[0] != list_tag:
                    close_list()
                    open_list = (list_tag, ilvl)
                    out.append('<%%s style="padding-left:2em;margin:8px 0">' %% list_tag)
                # 渲染列表项
                imgs = extract_images(para)
                runs_html = ''.join(render_run(r) for r in para.runs)
                inline = imgs + runs_html
                out.append('<li style="%%s">%%s</li>' %% (align_style, inline if inline.strip() else '&nbsp;'))
                continue
            else:
                close_list()

            # 判断标题
            style_name = para.style.name if para.style else ''
            heading_level = None
            if style_name and ('Heading' in style_name or 'heading' in style_name.lower()):
                try:
                    heading_level = int(''.join(c for c in style_name if c.isdigit()) or '2')
                except:
                    heading_level = 2

            imgs = extract_images(para)
            runs_html = ''.join(render_run(r) for r in para.runs)
            inline = imgs + runs_html

            if heading_level:
                htag = 'h%%d' %% min(heading_level, 4)
                out.append('<%%s style="%%s">%%s</%%s>' %% (htag, align_style, inline or '&nbsp;', htag))
            elif inline.strip() or imgs:
                out.append('<p style="%%s">%%s</p>' %% (align_style, inline))
            else:
                out.append('<p style="%%s"><br></p>' %% align_style)

        elif tag == 'tbl':
            close_list()
            tbl_index = len([c for c in body if c is child and c.tag.split('}')[-1] == 'tbl' or
                             (c.tag.split('}')[-1] == 'tbl' and list(body).index(c) < list(body).index(child))])
            # 简化：用遍历计数
            tbl_count = 0
            for c in body:
                if c is child:
                    break
                if c.tag.split('}')[-1] == 'tbl':
                    tbl_count += 1
            if tbl_count < len(doc.tables):
                table = doc.tables[tbl_count]
                out.append('<div style="overflow-x:auto;margin:12px 0"><table>')
                for r_idx, row in enumerate(table.rows):
                    is_header = (r_idx == 0)
                    if is_header:
                        out.append('<thead><tr>')
                    else:
                        if r_idx == 1:
                            out.append('</thead><tbody>')
                        out.append('<tr>')
                    for cell in row.cells:
                        ct = html_mod.escape(cell.text.strip())
                        tag_cell = 'th' if is_header else 'td'
                        out.append('<%%s>%%s</%%s>' %% (tag_cell, ct or '&nbsp;', tag_cell))
                    out.append('</tr>')
                out.append('</tbody></table></div>')

        elif tag == 'sdt':
            # 结构化文档标签 — 递归处理内部段落
            for inner_p in child.iter('{%%s}p' %% W_NS):
                pass  # 简单跳过，复杂 sdt 由段落循环覆盖

    close_list()
    out.append('</div>')
    print(''.join(out))
    sys.exit(0)

except ImportError:
    pass
except Exception:
    pass

# ========== 方案 2: zipfile + XML 直接解析（零依赖回退）==========
try:
    import zipfile, xml.etree.ElementTree as ET
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('word/document.xml') as f:
            tree = ET.parse(f)
        root = tree.getroot()
        ns = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'

        # 提取图片
        images = {}
        try:
            rels_xml = z.read('word/_rels/document.xml.rels')
            rels_tree = ET.fromstring(rels_xml)
            rns_x = 'http://schemas.openxmlformats.org/package/2006/relationships'
            for rel in rels_tree:
                rid = rel.get('Id', '')
                target = rel.get('Target', '')
                if 'image' in target.lower() or any(target.lower().endswith(e) for e in ('.png','.jpg','.jpeg','.gif','.bmp','.webp')):
                    img_path = 'word/' + target.replace('../', '')
                    try:
                        img_data = base64.b64encode(z.read(img_path)).decode()
                        ext = target.rsplit('.',1)[-1].lower()
                        if ext == 'jpeg': ext = 'jpg'
                        images[rid] = 'data:image/%%s;base64,%%s' %% (ext, img_data)
                    except:
                        pass
        except:
            pass

        out = [CSS, '<div class="docx-body">']
        for p in root.iter('{%%s}p' %% ns):
            texts = []
            has_img = False
            for r in p.iter('{%%s}r' %% ns):
                t = r.find('{%%s}t' %% ns)
                if t is not None and t.text:
                    texts.append(html_mod.escape(t.text))
                for drawing in r.iter('{%%s}drawing' %% ns):
                    for blip in drawing.iter('{%%s}blip' %% 'http://schemas.openxmlformats.org/drawingml/2006/main'):
                        embed = blip.get('{%%s}embed' %% 'http://schemas.openxmlformats.org/officeDocument/2006/relationships')
                        if embed and embed in images:
                            out.append('<img src="%%s" alt="">' %% images[embed])
                            has_img = True
            line = ''.join(texts)
            if has_img:
                pass
            elif line.strip():
                out.append('<p>%%s</p>' %% line)
            else:
                out.append('<p><br></p>')
        out.append('</div>')
        print(''.join(out))
        sys.exit(0)
except Exception:
    pass

print('<p style="color:#9d9ea4;padding:32px;text-align:center">⚠️ 无法预览此 .docx 文件<br><small>请安装 python-docx 获得最佳预览效果：<code>pip3 install python-docx</code></small></p>')
`, filePath)
}

// xlsxToHTMLScript 生成将 .xlsx 转换为 HTML 的 Python 脚本。
// 使用 openpyxl 解析单元格格式，表头行自动加重底色，
// 交替行着色、列宽适配，回退到 zipfile+XML。
func xlsxToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, json, html as html_mod
filepath = %q

CSS = '''<style>
.xlsx-body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#e4e5e9;line-height:1.6;padding:8px 0}
.xlsx-sheet{margin-bottom:24px}
.xlsx-sheet h4{font-size:13px;font-weight:600;color:#a0a2ab;margin:0 0 8px;padding:6px 12px;background:rgba(255,255,255,0.04);border-radius:6px;display:inline-block;letter-spacing:.03em}
.xlsx-table{width:100%%;border-collapse:collapse;font-size:13px;border:1px solid rgba(255,255,255,0.1);border-radius:8px;overflow:hidden;table-layout:auto}
.xlsx-table th{background:rgba(255,255,255,0.09);color:#ebecee;font-weight:600;padding:9px 14px;text-align:left;border-bottom:2px solid rgba(255,255,255,0.14);font-size:12px;letter-spacing:.02em;white-space:nowrap;position:sticky;top:0}
.xlsx-table td{padding:7px 14px;border-bottom:1px solid rgba(255,255,255,0.05);color:#cfd1d6;white-space:nowrap;vertical-align:middle}
.xlsx-table tbody tr:nth-child(even){background:rgba(255,255,255,0.015)}
.xlsx-table tbody tr:nth-child(odd){background:rgba(255,255,255,0.005)}
.xlsx-table tbody tr:hover{background:rgba(255,255,255,0.05)}
.xlsx-table tr:last-child td{border-bottom:none}
.xlsx-warn{color:#9d9ea4;padding:20px;text-align:center;font-size:13px}
</style>'''

# ========== 方案 1: openpyxl（推荐）==========
try:
    from openpyxl import load_workbook
    from openpyxl.styles import Font, PatternFill, Alignment
    from openpyxl.utils import get_column_letter

    wb = load_workbook(filepath, read_only=True, data_only=True)
    out = [CSS, '<div class="xlsx-body">']
    MAX_ROWS = 300

    for sname in wb.sheetnames:
        ws = wb[sname]
        out.append('<div class="xlsx-sheet"><h4>📊 %%s</h4>' %% html_mod.escape(sname))
        out.append('<div style="overflow-x:auto;border-radius:8px"><table class="xlsx-table">')

        row_count = 0
        for row in ws.iter_rows(values_only=True):
            if row_count >= MAX_ROWS:
                colspan = len(row) if row else 1
                out.append('<tr><td colspan="%%d" style="text-align:center;color:#9d9ea4;padding:16px">… 表格过大，仅显示前 %%d 行</td></tr>' %% (colspan, MAX_ROWS))
                break
            is_header = (row_count == 0)
            tag = 'th' if is_header else 'td'
            out.append('<tr>')
            for cell in row:
                val = str(cell) if cell is not None else ''
                # 格式化数字
                if isinstance(cell, float):
                    if cell == int(cell):
                        val = str(int(cell))
                    else:
                        val = ('%%.4f' %% cell).rstrip('0').rstrip('.')
                out.append('<%%s>%%s</%%s>' %% (tag, html_mod.escape(val) if val else '&nbsp;', tag))
            out.append('</tr>')
            row_count += 1

        out.append('</table></div></div>')

    out.append('</div>')
    wb.close()
    print(''.join(out))
    sys.exit(0)
except ImportError:
    pass
except Exception:
    pass

# ========== 方案 2: zipfile + XML 直接解析（零依赖）==========
try:
    import zipfile, xml.etree.ElementTree as ET
    SS_NS = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'

    with zipfile.ZipFile(filepath, 'r') as z:
        # sheet 名称
        with z.open('xl/workbook.xml') as f:
            wb_tree = ET.parse(f)
        sheets = []
        for sheet in wb_tree.iter('{%%s}sheet' %% SS_NS):
            sname = sheet.get('name', 'Sheet')
            sheets.append(sname)

        # shared strings
        shared_strings = []
        try:
            with z.open('xl/sharedStrings.xml') as f:
                ss_tree = ET.parse(f)
            for si in ss_tree.iter('{%%s}t' %% SS_NS):
                if si.text:
                    shared_strings.append(si.text)
        except:
            pass

        # 列宽（用于设置 min-width）
        col_widths = {}  # sheet_idx -> {col_letter: width}
        for idx in range(len(sheets)):
            try:
                with z.open('xl/worksheets/sheet%%d.xml' %% (idx+1)) as f:
                    cols_tree = ET.parse(f)
                cw = {}
                for col in cols_tree.iter('{%%s}col' %% SS_NS):
                    mn = int(col.get('min', '1'))
                    mx = int(col.get('max', '1'))
                    w = float(col.get('width', '8'))
                    for c in range(mn, mx+1):
                        cw[c] = w
                col_widths[idx] = cw
            except:
                pass

        out = [CSS, '<div class="xlsx-body">']
        MAX_ROWS = 300

        for s_idx, sname in enumerate(sheets):
            sheet_file = 'xl/worksheets/sheet%%d.xml' %% (s_idx+1)
            try:
                with z.open(sheet_file) as f:
                    ws_tree = ET.parse(f)
            except:
                continue

            out.append('<div class="xlsx-sheet"><h4>📊 %%s</h4>' %% html_mod.escape(sname))
            out.append('<div style="overflow-x:auto;border-radius:8px"><table class="xlsx-table">')
            rows = list(ws_tree.iter('{%%s}row' %% SS_NS))
            cw = col_widths.get(s_idx, {})
            col_count = 0

            for r_idx, row in enumerate(rows):
                if r_idx >= MAX_ROWS:
                    out.append('<tr><td colspan="%%d" style="text-align:center;color:#9d9ea4;padding:16px">… 表格过大，仅显示前 %%d 行</td></tr>' %% (max(col_count,1), MAX_ROWS))
                    break
                is_header = (r_idx == 0)
                tag = 'th' if is_header else 'td'
                out.append('<tr>')

                cells_in_row = 0
                for cell in row.iter('{%%s}c' %% SS_NS):
                    val = ''
                    v = cell.find('{%%s}v' %% SS_NS)
                    if v is not None and v.text:
                        t = cell.get('t', '')
                        if t == 's':
                            try:
                                si = int(v.text)
                                if 0 <= si < len(shared_strings):
                                    val = shared_strings[si]
                            except:
                                val = v.text
                        else:
                            val = v.text
                    out.append('<%%s>%%s</%%s>' %% (tag, html_mod.escape(val) if val else '&nbsp;', tag))
                    cells_in_row += 1
                col_count = max(col_count, cells_in_row)
                out.append('</tr>')

            out.append('</table></div></div>')

        out.append('</div>')
        print(''.join(out))
        sys.exit(0)
except Exception:
    pass

print('<p class="xlsx-warn">⚠️ 无法预览此 .xlsx 文件<br><small>请安装 openpyxl 获得最佳预览效果：<code>pip3 install openpyxl</code></small></p>')
`, filePath)
}

// pptxToHTMLScript 生成将 .pptx 转换为 HTML 的 Python 脚本。
// 使用 python-pptx 解析幻灯片，提取标题、正文、表格和文本格式，
// 每张幻灯片渲染为独立卡片，回退到 zipfile+XML。
func pptxToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, base64, html as html_mod
filepath = %q

CSS = '''<style>
.pptx-body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#e4e5e9;line-height:1.7;padding:8px 0}
.pptx-slide{margin:16px 0;padding:20px 24px;border:1px solid rgba(255,255,255,0.1);border-radius:10px;background:rgba(255,255,255,0.015);transition:background .15s}
.pptx-slide:hover{background:rgba(255,255,255,0.03)}
.pptx-slide-header{display:flex;align-items:center;gap:10px;margin-bottom:14px;padding-bottom:10px;border-bottom:1px solid rgba(255,255,255,0.08)}
.pptx-slide-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;background:rgba(129,140,248,0.15);color:#a5b4fc;border-radius:50%%;font-size:12px;font-weight:700;flex-shrink:0}
.pptx-slide-title{font-size:1.15rem;font-weight:600;color:#ebecee;margin:0}
.pptx-body p{margin:4px 0;font-size:14px;color:#d4d5d9}
.pptx-body ul,.pptx-body ol{padding-left:2em;margin:6px 0}
.pptx-body li{margin-bottom:3px;font-size:14px;color:#d4d5d9}
.pptx-body table{width:100%%;border-collapse:collapse;margin:10px 0;font-size:13px;border:1px solid rgba(255,255,255,0.1);border-radius:8px;overflow:hidden}
.pptx-body th{background:rgba(255,255,255,0.08);color:#ebecee;font-weight:600;padding:8px 14px;text-align:left;border-bottom:1px solid rgba(255,255,255,0.14);font-size:12px}
.pptx-body td{padding:7px 14px;border-bottom:1px solid rgba(255,255,255,0.05);color:#cfd1d6;vertical-align:top}
.pptx-body tbody tr:hover{background:rgba(255,255,255,0.025)}
.pptx-body tr:last-child td{border-bottom:none}
.pptx-body img{max-width:100%%;height:auto;border-radius:6px;margin:8px 0}
.pptx-warn{color:#9d9ea4;padding:32px;text-align:center;font-size:13px}
</style>'''

def esc(text):
    return html_mod.escape(str(text))

# ========== 方案 1: python-pptx（推荐）==========
try:
    from pptx import Presentation
    from pptx.util import Inches, Pt
    from pptx.enum.text import PP_ALIGN

    prs = Presentation(filepath)
    out = [CSS, '<div class="pptx-body">']

    ALIGN_MAP = {0:'left',1:'center',2:'right',3:'justify',None:'left'}

    for i, slide in enumerate(prs.slides, 1):
        title_text = ''
        body_parts = []

        for shape in slide.shapes:
            # 判断是否为标题
            is_title = shape.is_placeholder and shape.placeholder_format.idx == 0 if hasattr(shape, 'is_placeholder') else False

            if shape.has_text_frame:
                tf = shape.text_frame
                for para in tf.paragraphs:
                    # 构建行内格式
                    inline = ''
                    for run in para.runs:
                        t = esc(run.text or '')
                        if not t: continue
                        if run.font.bold:      t = '<strong>%%s</strong>' %% t
                        if run.font.italic:    t = '<em>%%s</em>' %% t
                        if run.font.underline: t = '<u>%%s</u>' %% t
                        if run.font.color and run.font.color.rgb:
                            t = '<span style="color:#%%s">%%s</span>' %% (str(run.font.color.rgb), t)
                        if run.font.size:
                            t = '<span style="font-size:%%.1fpx">%%s</span>' %% (run.font.size.pt, t)
                        inline += t

                    text = inline or esc(para.text or '')
                    if not text.strip():
                        continue

                    # 收集标题
                    if is_title and not title_text:
                        title_text = text
                    elif is_title:
                        body_parts.append('<p style="font-weight:600;font-size:15px">%%s</p>' %% text)
                    else:
                        # 判断是否为列表项（level 属性）
                        level = para.level if para.level else 0
                        bullet = para._element.find('.//{http://schemas.openxmlformats.org/drawingml/2006/main}buChar')
                        is_bullet = bullet is not None
                        if is_bullet:
                            indent = 1 + level
                            body_parts.append('<p style="padding-left:%%dem">• %%s</p>' %% (indent * 1.5, text))
                        else:
                            body_parts.append('<p>%%s</p>' %% text)

            if shape.has_table:
                table = shape.table
                html_tbl = ['<div style="overflow-x:auto;margin:10px 0"><table>']
                for r_idx, row in enumerate(table.rows):
                    is_hdr = (r_idx == 0)
                    tag = 'th' if is_hdr else 'td'
                    html_tbl.append('<tr>')
                    for cell in row.cells:
                        html_tbl.append('<%%s>%%s</%%s>' %% (tag, esc(cell.text.strip()) or '&nbsp;', tag))
                    html_tbl.append('</tr>')
                html_tbl.append('</table></div>')
                body_parts.append(''.join(html_tbl))

        # 渲染幻灯片卡片
        display_title = title_text or ('幻灯片 %%d' %% i)
        out.append('<div class="pptx-slide">')
        out.append('<div class="pptx-slide-header"><span class="pptx-slide-num">%%d</span><span class="pptx-slide-title">%%s</span></div>' %% (i, display_title))
        out.append(''.join(body_parts))
        out.append('</div>')

    out.append('</div>')
    print(''.join(out))
    sys.exit(0)

except ImportError:
    pass
except Exception:
    pass

# ========== 方案 2: zipfile + XML 直接解析（零依赖）==========
try:
    import zipfile, xml.etree.ElementTree as ET
    A_NS = 'http://schemas.openxmlformats.org/drawingml/2006/main'
    R_NS = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'

    with zipfile.ZipFile(filepath, 'r') as z:
        slides = sorted([n for n in z.namelist() if n.startswith('ppt/slides/slide') and n.endswith('.xml')],
                        key=lambda x: int(x.split('slide')[1].split('.')[0]) if x.split('slide')[1].split('.')[0].isdigit() else 0)

        # slide rels -> 图片
        slide_images = {}
        for sname in slides:
            rels_name = sname.replace('slides/', 'slides/_rels/') + '.rels'
            try:
                rels_xml = z.read(rels_name)
                rels_tree = ET.fromstring(rels_xml)
                imgs = {}
                for rel in rels_tree:
                    rid = rel.get('Id', '')
                    target = rel.get('Target', '')
                    if 'image' in target.lower() or any(target.lower().endswith(e) for e in ('.png','.jpg','.jpeg','.gif','.bmp')):
                        img_path = 'ppt/slides/' + target.replace('../', '')
                        try:
                            img_data = base64.b64encode(z.read(img_path)).decode()
                            ext = target.rsplit('.',1)[-1].lower()
                            if ext == 'jpeg': ext = 'jpg'
                            imgs[rid] = 'data:image/%%s;base64,%%s' %% (ext, img_data)
                        except:
                            pass
                slide_images[sname] = imgs
            except:
                pass

        out = [CSS, '<div class="pptx-body">']

        for i, sname in enumerate(slides, 1):
            with z.open(sname) as f:
                tree = ET.parse(f)
            root = tree.getroot()
            imgs = slide_images.get(sname, {})

            texts = []
            for t in root.iter('{%%s}t' %% A_NS):
                if t.text and t.text.strip():
                    texts.append(esc(t.text.strip()))

            # 提取图片
            img_tags = []
            for blip in root.iter('{%%s}blip' %% A_NS):
                embed = blip.get('{%%s}embed' %% R_NS)
                if embed and embed in imgs:
                    img_tags.append('<img src="%%s" alt="">' %% imgs[embed])

            title = texts[0] if texts else ''
            body = texts[1:] if len(texts) > 1 else []

            out.append('<div class="pptx-slide">')
            out.append('<div class="pptx-slide-header"><span class="pptx-slide-num">%%d</span><span class="pptx-slide-title">%%s</span></div>' %% (i, title or '幻灯片 %%d' %% i))
            out.append(''.join(img_tags))
            for line in body:
                out.append('<p>%%s</p>' %% line)
            out.append('</div>')

        out.append('</div>')
        print(''.join(out))
        sys.exit(0)
except Exception:
    pass

print('<p class="pptx-warn">⚠️ 无法预览此 .pptx 文件<br><small>请安装 python-pptx 获得最佳预览效果：<code>pip3 install python-pptx</code></small></p>')
`, filePath)
}

// odtToHTMLScript 生成将 .odt 转换为 HTML 的 Python 脚本。
// 解析 OpenDocument content.xml，提取标题、段落、文本格式和表格。
func odtToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, zipfile, xml.etree.ElementTree as ET, html as html_mod, base64
filepath = %q

CSS = '''<style>
.odt-body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#e4e5e9;line-height:1.85;font-size:14px;padding:8px 0}
.odt-body h1{font-size:1.55rem;font-weight:700;margin:24px 0 12px;padding-bottom:8px;border-bottom:1px solid rgba(255,255,255,0.1);color:#f0f1f3}
.odt-body h2{font-size:1.3rem;font-weight:600;margin:20px 0 10px;color:#ebecee}
.odt-body h3{font-size:1.14rem;font-weight:600;margin:16px 0 8px;color:#e4e5e9}
.odt-body p{margin:6px 0}
.odt-body table{width:100%%;border-collapse:collapse;margin:14px 0;font-size:13px;border:1px solid rgba(255,255,255,0.1);border-radius:8px;overflow:hidden}
.odt-body th{background:rgba(255,255,255,0.08);color:#ebecee;font-weight:600;padding:10px 14px;text-align:left;border-bottom:1px solid rgba(255,255,255,0.14);font-size:12px}
.odt-body td{padding:9px 14px;border-bottom:1px solid rgba(255,255,255,0.06);color:#cfd1d6;vertical-align:top}
.odt-body tbody tr:hover{background:rgba(255,255,255,0.025)}
.odt-body tr:last-child td{border-bottom:none}
.odt-body ul,.odt-body ol{padding-left:2em;margin:8px 0}
.odt-body li{margin-bottom:4px}
.odt-body img{max-width:100%%;height:auto;border-radius:6px;margin:10px 0}
.odt-body blockquote{border-left:3px solid #818cf8;padding:6px 16px;margin:12px 0;background:rgba(129,140,248,0.06);border-radius:0 6px 6px 0;color:#b0b3be}
.odt-body code{background:rgba(255,255,255,0.08);padding:2px 6px;border-radius:4px;font-size:.89em;font-family:"SF Mono","Fira Code",Menlo,monospace;color:#f87171}
</style>'''

TEXT_NS = 'urn:oasis:names:tc:opendocument:xmlns:text:1.0'
TABLE_NS = 'urn:oasis:names:tc:opendocument:xmlns:table:1.0'
OFFICE_NS = 'urn:oasis:names:tc:opendocument:xmlns:office:1.0'
STYLE_NS = 'urn:oasis:names:tc:opendocument:xmlns:style:1.0'
FO_NS = 'urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0'

try:
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('content.xml') as f:
            tree = ET.parse(f)
    root = tree.getroot()
    out = [CSS, '<div class="odt-body">']

    # 解析 heading 样式映射
    heading_styles = set()
    for style in root.iter('{%%s}style' %% STYLE_NS):
        name = style.get('{%%s}name' %% STYLE_NS, '')
        parent = style.get('{%%s}parent-style-name' %% STYLE_NS, '')
        if 'heading' in (name + parent).lower() or 'Heading' in (name + parent):
            heading_styles.add(name)

    def get_text_content(elem):
        """递归获取元素中所有文本内容，保留格式。"""
        parts = []
        # 检测粗体/斜体
        for span in elem.iter('{%%s}span' %% TEXT_NS):
            t = span.text or ''
            ts = span.get('{%%s}style-name' %% TEXT_NS, '')
            if 'bold' in ts.lower() or 'Bold' in ts:
                t = '<strong>%%s</strong>' %% html_mod.escape(t)
            elif 'italic' in ts.lower() or 'Italic' in ts:
                t = '<em>%%s</em>' %% html_mod.escape(t)
            elif 'underline' in ts.lower():
                t = '<u>%%s</u>' %% html_mod.escape(t)
            else:
                t = html_mod.escape(t)
            parts.append(t)
        if not parts and elem.text:
            parts.append(html_mod.escape(elem.text))
        return ''.join(parts)

    # 遍历 body
    body = root.find('{%%s}body' %% OFFICE_NS)
    if body is None:
        body = root

    for child in body:
        tag = child.tag.split('}')[-1] if '}' in child.tag else child.tag

        if tag == 'p':
            style_name = child.get('{%%s}style-name' %% TEXT_NS, '')
            content = get_text_content(child)
            if style_name in heading_styles:
                # 提取 heading 级别
                lvl = 2
                for c in style_name:
                    if c.isdigit():
                        lvl = int(c)
                        break
                out.append('<h%%d>%%s</h%%d>' %% (min(lvl, 4), content or '&nbsp;', min(lvl, 4)))
            elif content.strip():
                out.append('<p>%%s</p>' %% content)
            else:
                out.append('<p><br></p>')

        elif tag == 'table':
            out.append('<div style="overflow-x:auto;margin:12px 0"><table>')
            rows = list(child.iter('{%%s}table-row' %% TABLE_NS))
            for r_idx, row in enumerate(rows):
                is_header = (r_idx == 0)
                t = 'th' if is_header else 'td'
                out.append('<tr>')
                for cell in row.iter('{%%s}table-cell' %% TABLE_NS):
                    cell_text = get_text_content(cell)
                    out.append('<%%s>%%s</%%s>' %% (t, cell_text or '&nbsp;', t))
                out.append('</tr>')
            out.append('</table></div>')

        elif tag in ('h', 'text:h'):
            lvl = child.get('{%%s}outline-level' %% TEXT_NS, '2')
            content = get_text_content(child)
            out.append('<h%%d>%%s</h%%d>' %% (min(int(lvl), 4), content or '&nbsp;', min(int(lvl), 4)))

        elif tag == 'list':
            out.append('<ul style="padding-left:2em;margin:8px 0">')
            for item in child.iter('{%%s}list-item' %% TEXT_NS):
                for p in item.iter('{%%s}p' %% TEXT_NS):
                    content = get_text_content(p)
                    if content.strip():
                        out.append('<li>%%s</li>' %% content)
            out.append('</ul>')

    out.append('</div>')
    print(''.join(out))
    sys.exit(0)
except Exception as e:
    print('<p style="color:#9d9ea4;padding:32px;text-align:center">⚠️ 无法预览此 .odt 文件<br><small>%%s</small></p>' %% str(e))
`, filePath)
}

// odsToHTMLScript 生成将 .ods 转换为 HTML 的 Python 脚本。
// 解析 OpenDocument spreadsheet，表头加固、交替行着色。
func odsToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, zipfile, xml.etree.ElementTree as ET, html as html_mod
filepath = %q

CSS = '''<style>
.ods-body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#e4e5e9;line-height:1.6;padding:8px 0}
.ods-table{width:100%%;border-collapse:collapse;font-size:13px;border:1px solid rgba(255,255,255,0.1);border-radius:8px;overflow:hidden}
.ods-table th{background:rgba(255,255,255,0.09);color:#ebecee;font-weight:600;padding:9px 14px;text-align:left;border-bottom:2px solid rgba(255,255,255,0.14);font-size:12px;letter-spacing:.02em;white-space:nowrap}
.ods-table td{padding:7px 14px;border-bottom:1px solid rgba(255,255,255,0.05);color:#cfd1d6;white-space:nowrap;vertical-align:middle}
.ods-table tbody tr:nth-child(even){background:rgba(255,255,255,0.015)}
.ods-table tbody tr:nth-child(odd){background:rgba(255,255,255,0.005)}
.ods-table tbody tr:hover{background:rgba(255,255,255,0.05)}
.ods-table tr:last-child td{border-bottom:none}
</style>'''

TABLE_NS = 'urn:oasis:names:tc:opendocument:xmlns:table:1.0'
TEXT_NS  = 'urn:oasis:names:tc:opendocument:xmlns:text:1.0'

try:
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('content.xml') as f:
            tree = ET.parse(f)
    root = tree.getroot()
    out = [CSS, '<div class="ods-body"><div style="overflow-x:auto;border-radius:8px"><table class="ods-table">']

    rows = list(root.iter('{%%s}table-row' %% TABLE_NS))
    MAX_ROWS = 300
    for r_idx, row in enumerate(rows):
        if r_idx >= MAX_ROWS:
            out.append('<tr><td colspan="99" style="text-align:center;color:#9d9ea4;padding:16px">… 表格过大，仅显示前 %%d 行</td></tr>' %% MAX_ROWS)
            break
        is_header = (r_idx == 0)
        tag = 'th' if is_header else 'td'
        out.append('<tr>')
        for cell in row.iter('{%%s}table-cell' %% TABLE_NS):
            # 收集单元格内文本
            texts = []
            for p in cell.iter('{%%s}p' %% TEXT_NS):
                if p.text:
                    texts.append(html_mod.escape(p.text))
            val = ' '.join(texts)
            out.append('<%%s>%%s</%%s>' %% (tag, val if val else '&nbsp;', tag))
        out.append('</tr>')

    out.append('</table></div></div>')
    print(''.join(out))
    sys.exit(0)
except Exception as e:
    print('<p style="color:#9d9ea4;padding:32px;text-align:center">⚠️ 无法预览此 .ods 文件<br><small>%%s</small></p>' %% str(e))
`, filePath)
}