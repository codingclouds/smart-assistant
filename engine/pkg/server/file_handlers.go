package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/smart-assistant/engine/internal/workspace"
)

// handleFileDownload 下载工作空间中的文件。
// GET /api/files/download?workspace_id=X&path=Y
func handleFileDownload(workspaceSvc *workspace.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		filePath := r.URL.Query().Get("path")
		if workspaceID == "" || filePath == "" {
			http.Error(w, "workspace_id and path are required", http.StatusBadRequest)
			return
		}

		fullPath, err := resolveWorkspaceFilePath(workspaceID, filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 读取文件内容
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to read file", http.StatusInternalServerError)
			}
			return
		}

		// 设置 Content-Type 和下载头
		fileName := filepath.Base(filePath)
		w.Header().Set("Content-Type", detectMimeType(filePath))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
	}
}

// handleFileContent 获取文件内容用于预览。
// GET /api/files/content?workspace_id=X&path=Y
func handleFileContent(workspaceSvc *workspace.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		filePath := r.URL.Query().Get("path")
		if workspaceID == "" || filePath == "" {
			http.Error(w, "workspace_id and path are required", http.StatusBadRequest)
			return
		}

		fullPath, err := resolveWorkspaceFilePath(workspaceID, filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to read file", http.StatusInternalServerError)
			}
			return
		}

		// 返回 JSON 格式：content_type, content (text) 或 base64 (binary)
		mimeType := detectMimeType(filePath)
		resp := map[string]interface{}{
			"file_name":    filepath.Base(filePath),
			"file_path":    filePath,
			"content_type": mimeType,
			"size":         len(data),
			"workspace_id": workspaceID,
		}

		if isTextMime(mimeType) {
			resp["content"] = string(data)
		} else {
			resp["content"] = "" // 二进制文件不在 JSON 中传输，用 download 端点
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handleFileList 列出工作空间中的文件。
// GET /api/files/list?workspace_id=X
func handleFileList(workspaceSvc *workspace.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			http.Error(w, "workspace_id is required", http.StatusBadRequest)
			return
		}

		filesDir := workspace.FilesDir(workspaceID)
		var files []map[string]interface{}

		err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
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

		if err != nil {
			log.Printf("[files] walk error: %v", err)
			// 目录不存在时返回空列表
			files = []map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}
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

// handleFilePreview 将 Office 文档（.docx/.xlsx/.pptx 等）转换为 HTML 用于预览。
// GET /api/files/preview?workspace_id=X&path=Y
func handleFilePreview(workspaceSvc *workspace.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		filePath := r.URL.Query().Get("path")
		if workspaceID == "" || filePath == "" {
			http.Error(w, "workspace_id and path are required", http.StatusBadRequest)
			return
		}

		fullPath, err := resolveWorkspaceFilePath(workspaceID, filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 检查文件是否存在
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to stat file", http.StatusInternalServerError)
			}
			return
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		resp := map[string]interface{}{
			"file_name":         filepath.Base(filePath),
			"file_path":         filePath,
			"content_type":      detectMimeType(filePath),
			"size":              info.Size(),
			"workspace_id":      workspaceID,
			"preview_available": false,
			"preview_html":      "",
			"preview_error":     "",
		}

		if !isOfficeDocument(filePath) {
			resp["preview_error"] = fmt.Sprintf("preview not supported for %s files", ext)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// 尝试通过 Python 转换 Office 文档为 HTML
		html, convErr := convertOfficeToHTML(fullPath, ext)
		if convErr != nil {
			resp["preview_error"] = convErr.Error()
			log.Printf("[files] preview conversion failed for %s: %v", filePath, convErr)
		} else {
			resp["preview_available"] = true
			resp["preview_html"] = html
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// convertOfficeToHTML 使用 Python 将 Office 文档转换为 HTML。
// 支持 .docx（python-docx）、.xlsx（openpyxl）、.pptx（python-pptx）。
func convertOfficeToHTML(filePath, ext string) (string, error) {
	var script string
	switch ext {
	case ".docx":
		script = docxToHTMLScript(filePath)
	case ".xlsx":
		script = xlsxToHTMLScript(filePath)
	case ".pptx", ".odp":
		script = pptxToHTMLScript(filePath)
	case ".odt":
		script = odtToHTMLScript(filePath)
	case ".ods":
		script = odsToHTMLScript(filePath)
	default:
		return "", fmt.Errorf("unsupported format: %s", ext)
	}

	cmd := exec.Command("python3", "-c", script)
	cmd.Stderr = nil // stderr goes to default
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("python3 conversion failed: %v — stderr: %s", err, stderrBuf.String())
	}

	result := stdoutBuf.String()
	if result == "" {
		return "", fmt.Errorf("python3 produced empty output — stderr: %s", stderrBuf.String())
	}
	return result, nil
}

// docxToHTMLScript 生成将 .docx 转换为 HTML 的 Python 脚本。
func docxToHTMLScript(filePath string) string {
	// 先尝试 python-docx，失败则尝试 docx2txt，再失败则用 zipfile 直接读 XML
	return fmt.Sprintf(`
import sys, os
filepath = %q

# 方案 1: python-docx
try:
    from docx import Document
    doc = Document(filepath)
    print('<div class="docx-preview">')
    for para in doc.paragraphs:
        text = para.text or ''
        style = para.style.name if para.style else ''
        if style and 'Heading' in style:
            level = style.replace('Heading ', '').strip()
            if level.isdigit():
                print(f'<h{level}>{text}</h{level}>')
            else:
                print(f'<h2>{text}</h2>')
        elif text.strip():
            print(f'<p>{text}</p>')
        else:
            print(f'<p><br></p>')
    # 表格
    for table in doc.tables:
        print('<table border="1" style="border-collapse:collapse;width:100%%;margin:8px 0">')
        for row in table.rows:
            print('<tr>')
            for cell in row.cells:
                print(f'<td style="padding:4px 8px">{cell.text}</td>')
            print('</tr>')
        print('</table>')
    print('</div>')
    sys.exit(0)
except ImportError:
    pass

# 方案 2: docx2txt
try:
    import docx2txt
    text = docx2txt.process(filepath)
    print('<div class="docx-preview" style="white-space:pre-wrap;font-family:system-ui,sans-serif;line-height:1.6">')
    print(text.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;'))
    print('</div>')
    sys.exit(0)
except ImportError:
    pass

# 方案 3: 直接用 zipfile 提取 document.xml
try:
    import zipfile, xml.etree.ElementTree as ET
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('word/document.xml') as f:
            tree = ET.parse(f)
        root = tree.getroot()
        ns = {'w': 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'}
        print('<div class="docx-preview">')
        for p in root.iter('{http://schemas.openxmlformats.org/wordprocessingml/2006/main}p'):
            texts = []
            for t in p.iter('{http://schemas.openxmlformats.org/wordprocessingml/2006/main}t'):
                if t.text:
                    texts.append(t.text)
            line = ''.join(texts)
            if line.strip():
                print(f'<p>{line}</p>')
            else:
                print('<p><br></p>')
        print('</div>')
        sys.exit(0)
except Exception as e:
    pass

print(f'<p style="color:var(--color-text-muted)">无法预览此 .docx 文件。请安装 python-docx：pip3 install python-docx</p>')
`, filePath)
}

// xlsxToHTMLScript 生成将 .xlsx 转换为 HTML 的 Python 脚本。
func xlsxToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, json
filepath = %q

# 方案 1: openpyxl
try:
    from openpyxl import load_workbook
    wb = load_workbook(filepath, read_only=True, data_only=True)
    print('<div class="xlsx-preview">')
    for sname in wb.sheetnames:
        ws = wb[sname]
        print(f'<h3 style="margin:12px 0 4px">{sname}</h3>')
        print('<div style="overflow-x:auto"><table border="1" style="border-collapse:collapse;font-size:12px">')
        row_count = 0
        for row in ws.iter_rows(values_only=True):
            if row_count > 200:
                print('<tr><td colspan="100" style="padding:8px;color:var(--color-text-muted)">... 表格过大，仅显示前 200 行</td></tr>')
                break
            print('<tr>')
            for cell in row:
                val = str(cell) if cell is not None else ''
                print(f'<td style="padding:2px 6px;white-space:nowrap">{val}</td>')
            print('</tr>')
            row_count += 1
        print('</table></div>')
    print('</div>')
    wb.close()
    sys.exit(0)
except ImportError:
    pass

# 方案 2: 用 zipfile + xml 直接解析
try:
    import zipfile, xml.etree.ElementTree as ET
    with zipfile.ZipFile(filepath, 'r') as z:
        # 先读 workbook.xml 获取 sheet 名称
        with z.open('xl/workbook.xml') as f:
            wb_tree = ET.parse(f)
        ns = {'s': 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'}
        sheets = []
        for sheet in wb_tree.iter('{http://schemas.openxmlformats.org/spreadsheetml/2006/main}sheet'):
            sname = sheet.get('name', 'Sheet')
            sheets.append(sname)

        # 读取 shared strings
        shared_strings = []
        try:
            with z.open('xl/sharedStrings.xml') as f:
                ss_tree = ET.parse(f)
            for si in ss_tree.iter('{http://schemas.openxmlformats.org/spreadsheetml/2006/main}t'):
                if si.text:
                    shared_strings.append(si.text)
        except:
            pass

        print('<div class="xlsx-preview">')
        for idx, sname in enumerate(sheets):
            sheet_file = f'xl/worksheets/sheet{idx+1}.xml'
            try:
                with z.open(sheet_file) as f:
                    ws_tree = ET.parse(f)
            except:
                continue
            print(f'<h3 style="margin:12px 0 4px">{sname}</h3>')
            print('<div style="overflow-x:auto"><table border="1" style="border-collapse:collapse;font-size:12px">')
            row_count = 0
            for row in ws_tree.iter('{http://schemas.openxmlformats.org/spreadsheetml/2006/main}row'):
                if row_count > 200:
                    print('<tr><td colspan="100" style="padding:8px;color:var(--color-text-muted)">... 表格过大，仅显示前 200 行</td></tr>')
                    break
                print('<tr>')
                for cell in row.iter('{http://schemas.openxmlformats.org/spreadsheetml/2006/main}c'):
                    val = ''
                    v = cell.find('{http://schemas.openxmlformats.org/spreadsheetml/2006/main}v')
                    if v is not None and v.text:
                        t = cell.get('t', '')
                        if t == 's':
                            try:
                                idx = int(v.text)
                                if idx < len(shared_strings):
                                    val = shared_strings[idx]
                            except:
                                val = v.text
                        else:
                            val = v.text
                    print(f'<td style="padding:2px 6px;white-space:nowrap">{val}</td>')
                print('</tr>')
                row_count += 1
            print('</table></div>')
        print('</div>')
        sys.exit(0)
except Exception as e:
    pass

print(f'<p style="color:var(--color-text-muted)">无法预览此 .xlsx 文件。请安装 openpyxl：pip3 install openpyxl</p>')
`, filePath)
}

// pptxToHTMLScript 生成将 .pptx 转换为 HTML 的 Python 脚本。
func pptxToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys
filepath = %q

# 方案 1: python-pptx
try:
    from pptx import Presentation
    prs = Presentation(filepath)
    print('<div class="pptx-preview">')
    for i, slide in enumerate(prs.slides, 1):
        print(f'<div style="margin:16px 0;padding:12px;border:1px solid var(--color-border);border-radius:8px">')
        print(f'<h3 style="margin:0 0 8px">幻灯片 {i}</h3>')
        for shape in slide.shapes:
            if shape.has_text_frame:
                for para in shape.text_frame.paragraphs:
                    text = para.text or ''
                    if text.strip():
                        print(f'<p style="margin:4px 0">{text}</p>')
            if shape.has_table:
                table = shape.table
                print('<table border="1" style="border-collapse:collapse;width:100%%;margin:8px 0;font-size:12px">')
                for row in table.rows:
                    print('<tr>')
                    for cell in row.cells:
                        print(f'<td style="padding:2px 6px">{cell.text}</td>')
                    print('</tr>')
                print('</table>')
        print('</div>')
    print('</div>')
    sys.exit(0)
except ImportError:
    pass

# 方案 2: 用 zipfile 直接读 XML
try:
    import zipfile, xml.etree.ElementTree as ET
    with zipfile.ZipFile(filepath, 'r') as z:
        slides = sorted([n for n in z.namelist() if n.startswith('ppt/slides/slide') and n.endswith('.xml')])
        print('<div class="pptx-preview">')
        for i, sname in enumerate(slides, 1):
            with z.open(sname) as f:
                tree = ET.parse(f)
            root = tree.getroot()
            ns = {'a': 'http://schemas.openxmlformats.org/drawingml/2006/main'}
            texts = []
            for t in root.iter('{http://schemas.openxmlformats.org/drawingml/2006/main}t'):
                if t.text:
                    texts.append(t.text)
            if texts:
                print(f'<div style="margin:16px 0;padding:12px;border:1px solid var(--color-border);border-radius:8px">')
                print(f'<h3 style="margin:0 0 8px">幻灯片 {i}</h3>')
                for t in texts:
                    print(f'<p style="margin:4px 0">{t}</p>')
                print('</div>')
        print('</div>')
        sys.exit(0)
except Exception as e:
    pass

print(f'<p style="color:var(--color-text-muted)">无法预览此 .pptx 文件。请安装 python-pptx：pip3 install python-pptx</p>')
`, filePath)
}

// odtToHTMLScript 生成将 .odt 转换为 HTML 的 Python 脚本。
func odtToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, zipfile, xml.etree.ElementTree as ET
filepath = %q
try:
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('content.xml') as f:
            tree = ET.parse(f)
    root = tree.getroot()
    ns = {'text': 'urn:oasis:names:tc:opendocument:xmlns:text:1.0',
          'office': 'urn:oasis:names:tc:opendocument:xmlns:office:1.0'}
    print('<div class="odt-preview">')
    for p in root.iter('{urn:oasis:names:tc:opendocument:xmlns:text:1.0}p'):
        texts = []
        for span in p.iter('{urn:oasis:names:tc:opendocument:xmlns:text:1.0}span'):
            if span.text:
                texts.append(span.text)
        line = ''.join(texts) or (p.text or '')
        if line.strip():
            print(f'<p>{line}</p>')
        else:
            print('<p><br></p>')
    print('</div>')
    sys.exit(0)
except Exception as e:
    print(f'<p style="color:var(--color-text-muted)">无法预览此 .odt 文件：{e}</p>')
`, filePath)
}

// odsToHTMLScript 生成将 .ods 转换为 HTML 的 Python 脚本。
func odsToHTMLScript(filePath string) string {
	return fmt.Sprintf(`
import sys, zipfile, xml.etree.ElementTree as ET
filepath = %q
try:
    with zipfile.ZipFile(filepath, 'r') as z:
        with z.open('content.xml') as f:
            tree = ET.parse(f)
    root = tree.getroot()
    print('<div class="ods-preview"><div style="overflow-x:auto"><table border="1" style="border-collapse:collapse;font-size:12px">')
    for row in root.iter('{urn:oasis:names:tc:opendocument:xmlns:table:1.0}table-row'):
        print('<tr>')
        for cell in row.iter('{urn:oasis:names:tc:opendocument:xmlns:table:1.0}table-cell'):
            texts = []
            for p in cell.iter('{urn:oasis:names:tc:opendocument:xmlns:text:1.0}p'):
                if p.text:
                    texts.append(p.text)
            val = ' '.join(texts)
            print(f'<td style="padding:2px 6px">{val}</td>')
        print('</tr>')
    print('</table></div></div>')
    sys.exit(0)
except Exception as e:
    print(f'<p style="color:var(--color-text-muted)">无法预览此 .ods 文件：{e}</p>')
`, filePath)
}