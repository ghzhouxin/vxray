<template>
  <AppDialog :model-value="modelValue" title="Xray 配置" align-center class="xray-config-dialog" @update:model-value="emit('update:modelValue', $event)">
    <div class="editor-toolbar">
      <span class="editor-dirty" :class="{ active: dirty }">{{ dirty ? '● 未保存' : '已同步' }}</span>
      <div class="editor-actions">
        <IconButton tooltip="格式化 JSON" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="handleFormat"><Operation /></IconButton>
        <IconButton tooltip="校验 JSON" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="validate"><CircleCheck /></IconButton>
        <span class="action-divider"></span>
        <IconButton tooltip="恢复默认配置" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="handleResetDefault"><RefreshLeft /></IconButton>
        <IconButton tooltip="回滚到已保存版本" :size="ICON_BUTTON_SIZE_SM" tone="muted" @click="handleRevert"><Back /></IconButton>
        <span class="action-divider"></span>
        <IconButton tooltip="保存配置" :size="ICON_BUTTON_SIZE_SM" tone="primary" :working="xrayConfigStore.saving" :disabled="xrayConfigStore.saving" @click="save"><Check /></IconButton>
      </div>
    </div>
    <div v-if="jsonError" class="editor-error-bar">⚠ {{ jsonError }}</div>
    <div ref="editorRef" class="codemirror-host"></div>
  </AppDialog>
</template>

<script setup lang="ts">
import { json } from '@codemirror/lang-json'
import { EditorState } from '@codemirror/state'
import { codeFolding, ensureSyntaxTree, foldEffect, foldGutter, foldable, syntaxTree } from '@codemirror/language'
import { EditorView } from '@codemirror/view'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Back, Check, CircleCheck, Operation, RefreshLeft } from '@element-plus/icons-vue'
import { useXrayConfigStore } from '@/stores'
import { CODEMIRROR_PARSE_TIMEOUT_MS, ICON_BUTTON_SIZE_SM } from '@/constants'
import { formatJsonString, formatJsonError } from '@/utils/formatters'
import { msg } from '@/utils/message'
import AppDialog from '@/components/AppDialog.vue'
import IconButton from '@/components/IconButton.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: []
  saved: []
}>()

const xrayConfigStore = useXrayConfigStore()
const dirty = computed(() => xrayConfigStore.xrayConfigText !== xrayConfigStore.originalXrayConfigText)

const editorRef = ref<HTMLElement>()
const jsonError = ref('')
let editor: EditorView | null = null

const editorTheme = EditorView.theme({
  '&': { height: '100%', backgroundColor: 'var(--bg-input)', color: 'var(--text-primary)', fontSize: 'var(--font-sm)', lineHeight: '1.65' },
  '.cm-scroller': { fontFamily: 'var(--mono)', overscrollBehavior: 'contain' },
  '.cm-activeLine': { backgroundColor: 'var(--bg-hover)' },
  '.cm-content': { caretColor: 'var(--primary)' },
  '.cm-gutters': {
    backgroundColor: 'var(--bg-secondary)',
    color: 'var(--text-tertiary)',
    borderRight: '1px solid var(--border-soft)',
    minWidth: '20px'
  },
  '.cm-foldGutter .cm-gutterElement': {
    width: '20px', padding: '0', display: 'flex', alignItems: 'center',
    justifyContent: 'center', cursor: 'pointer', fontSize: 'var(--font-xs)'
  },
  '.cm-foldPlaceholder': {
    backgroundColor: 'var(--bg-active)',
    border: '1px solid var(--border-active)',
    color: 'var(--primary)', borderRadius: 'var(--radius-xs)', padding: '0 var(--spacing-xs)'
  },
  '&.cm-focused': { outline: 'none' }
})

function ensureEditor() {
  if (editor || !editorRef.value) return
  editor = new EditorView({
    parent: editorRef.value,
    state: EditorState.create({
      doc: xrayConfigStore.xrayConfigText,
      extensions: [
        json(),
        codeFolding(),
        foldGutter({
          openText: '▾',
          closedText: '▸'
        }),
        editorTheme,
        EditorView.lineWrapping,
        EditorView.updateListener.of(update => {
          if (!update.docChanged) return
          const next = update.state.doc.toString()
          xrayConfigStore.xrayConfigText = next
          validateContent(next)
        }),
      ]
    })
  })
  validateContent(xrayConfigStore.xrayConfigText)
  queueMicrotask(foldTopLevelSections)
}

function destroyEditor() {
  editor?.destroy()
  editor = null
}

function syncEditorContent(value: string) {
  if (!editor || editor.state.doc.toString() === value) return
  editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: value } })
  queueMicrotask(foldTopLevelSections)
}

function validateContent(value: string) {
  try {
    JSON.parse(value)
    jsonError.value = ''
    return true
  } catch (error) {
    jsonError.value = formatJsonError(value, error)
    return false
  }
}

function validate() {
  validateContent(xrayConfigStore.xrayConfigText)
}

function handleFormat() {
  if (!validateContent(xrayConfigStore.xrayConfigText)) return
  xrayConfigStore.xrayConfigText = formatJsonString(xrayConfigStore.xrayConfigText)
}

function handleResetDefault() {
  xrayConfigStore.restoreDefaultXrayConfig()
  msg.success('已恢复默认配置，请保存以生效')
}

function handleRevert() {
  xrayConfigStore.resetXrayConfig()
}

async function save() {
  if (!validateContent(xrayConfigStore.xrayConfigText)) return
  emit('save')
  emit('saved')
}

function foldTopLevelSections() {
  if (!editor) return
  const effects = topLevelFoldRanges().map(range => foldEffect.of(range))
  if (effects.length) {
    editor.dispatch({ effects })
  }
}

function topLevelFoldRanges() {
  if (!editor) return []
  ensureSyntaxTree(editor.state, editor.state.doc.length, CODEMIRROR_PARSE_TIMEOUT_MS)
  const rootObject = syntaxTree(editor.state).topNode.getChild('Object')
  if (!rootObject) return []

  const ranges: Array<{ from: number; to: number }> = []
  const cursor = rootObject.cursor()
  if (!cursor.firstChild()) return ranges

  do {
    if (cursor.name !== 'Property') continue
    const propertyCursor = cursor.node.cursor()
    if (!propertyCursor.firstChild()) continue
    do {
      if (propertyCursor.name !== 'Object' && propertyCursor.name !== 'Array') continue
      const range = foldable(editor.state, propertyCursor.from + 1, propertyCursor.from + 1)
      if (range) ranges.push(range)
      break
    } while (propertyCursor.nextSibling())
  } while (cursor.nextSibling())

  return ranges
}

watch(
  () => props.modelValue,
  value => {
    if (value) {
      queueMicrotask(() => {
        ensureEditor()
        syncEditorContent(xrayConfigStore.xrayConfigText)
        queueMicrotask(foldTopLevelSections)
      })
      return
    }
    destroyEditor()
  },
  { immediate: true }
)

watch(() => xrayConfigStore.xrayConfigText, syncEditorContent)

onBeforeUnmount(destroyEditor)
</script>

<!-- 非 scoped：append-to-body 把 .el-dialog 挂到 body 下，脱离组件作用域，
     :deep() 无法命中。改用自定义类 + 全局样式覆盖 dialog 级布局。 -->
<style>
.xray-config-dialog.el-dialog {
  height: 90vh;
  max-height: 90vh;
  /* align-center 时 overlay 是 flex 容器，margin:auto 让 dialog 居中；
     用 !important 覆盖 .el-dialog 默认的 margin-top:15vh */
  margin: auto !important;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.xray-config-dialog .el-dialog__header,
.xray-config-dialog .el-dialog__footer {
  flex-shrink: 0;
}

/* body 不滚动，仅编辑器内部滚动；覆盖 global.css 的 max-height 与 overflow-y */
.xray-config-dialog .el-dialog__body {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  max-height: none;
  min-height: 360px;
  padding-bottom: var(--spacing-md) !important;
}
</style>

<style scoped>
.editor-toolbar {
  min-height: 40px;
  padding: 0 0 var(--spacing-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-md);
  flex-shrink: 0;
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
  justify-content: flex-end;
}

.action-divider {
  width: 1px;
  height: 20px;
  background: var(--border-soft);
  margin: 0 var(--spacing-xs);
}

.editor-dirty {
  color: var(--text-tertiary);
  font-size: var(--font-xs);
}

.editor-dirty.active {
  color: var(--warning);
}

.editor-error-bar {
  padding: 6px 12px;
  background: rgba(239, 68, 68, 0.08);
  border-bottom: 1px solid rgba(239, 68, 68, 0.2);
  color: var(--danger);
  font-size: var(--font-xs);
  flex-shrink: 0;
}

.codemirror-host {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 1;
  min-height: 240px;
  background: var(--bg-input);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-lg);
}
</style>
