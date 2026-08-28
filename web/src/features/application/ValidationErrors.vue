<script setup lang="ts">
import type { ApplicationError, ValidationResult } from "./types";
import { stageLabels } from "./types";

defineProps<{ validation: ValidationResult | null; error: ApplicationError | null }>();
const emit = defineEmits<{ jump: [path: string] }>();

function stageLabel(stage?: string): string {
  return stage ? stageLabels[stage] ?? stage : "配置";
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_@%+=:,./-]+$/.test(value)) return value;
  return `'${value.replaceAll("'", `'"'"'`)}'`;
}

function formatCommand(command?: string[]): string {
  return command?.map(shellQuote).join(" ") ?? "";
}
</script>

<template>
  <section v-if="validation || error" class="errors" :class="{ success: validation?.valid && !error }" :role="validation?.valid && !error ? 'status' : 'alert'">
    <h2>配置检查结果</h2>
    <p v-if="validation?.valid"><strong>{{ stageLabel(validation.stage) }}</strong>：{{ validation.message || "配置检查通过" }}</p>
    <ul v-if="validation?.errors?.length">
      <li v-for="item in validation.errors" :key="`${item.fieldPath}-${item.message}`">
        <button v-if="item.fieldPath" type="button" @click="emit('jump', item.fieldPath)">{{ item.fieldPath }}</button>
        <span v-else>配置</span>：{{ item.message }}
      </li>
    </ul>
    <p v-if="validation && !validation.valid"><strong>{{ stageLabel(validation.stage) }}</strong>：{{ validation.message }}</p>
    <p v-if="error">
      <strong>{{ stageLabel(error.stage) }}</strong>：{{ error.message }}
      <button v-if="error.fieldPath" type="button" @click="emit('jump', error.fieldPath)">{{ error.fieldPath }}</button>
    </p>

    <dl v-if="validation && !validation.valid && validation.configDirectory" class="check-details">
      <template v-if="validation.command?.length">
        <dt>实际检查命令</dt>
        <dd><code>{{ formatCommand(validation.command) }}</code></dd>
      </template>
      <dt>送检配置目录</dt>
      <dd><code>{{ validation.configDirectory }}</code><small>相对服务工作目录；该目录会保留，并在下次检查同一配置时更新。</small></dd>
      <template v-if="validation.files?.length">
        <dt>送检配置文件</dt>
        <dd><code>{{ validation.files.map((file) => file.name).join(", ") }}</code></dd>
      </template>
      <dt>草稿修订</dt>
      <dd>revision {{ validation.revision }}<code v-if="validation.digest">配置摘要 {{ validation.digest }}</code></dd>
      <dt>进程结果</dt>
      <dd>退出码 {{ validation.exitCode }}<span v-if="validation.timedOut">，执行超时</span></dd>
    </dl>

    <p v-if="validation?.outputTruncated" class="detail-warning">核心输出超过 64 KiB，以下内容已截断。</p>
    <div v-if="validation?.stdout" class="process-output"><h3>标准输出（stdout）</h3><pre>{{ validation.stdout }}</pre></div>
    <div v-if="validation?.stderr" class="process-output"><h3>标准错误（stderr）</h3><pre>{{ validation.stderr }}</pre></div>
    <pre v-if="error?.output">{{ error.output }}</pre>
  </section>
</template>

<style scoped>
.errors { margin-top: 16px; padding: 14px; border: 1px solid color-mix(in srgb, var(--red) 40%, var(--line)); border-radius: var(--radius); color: var(--red); background: var(--red-soft); }
.errors.success { border-color: color-mix(in srgb, var(--green) 40%, var(--line)); color: var(--green); background: var(--green-soft); }
.errors h2 { margin: 0; font-size: 16px; }
.errors p { margin: 10px 0 0; }
.check-details { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: 8px 14px; margin: 12px 0 0; color: var(--text); }
.check-details dt { font-weight: 700; color: var(--muted); }
.check-details dd { min-width: 0; margin: 0; }
.check-details code { display: block; overflow-wrap: anywhere; white-space: pre-wrap; }
.check-details small { display: block; margin-top: 3px; color: var(--muted); }
.detail-warning { font-weight: 700; }
.process-output { margin-top: 12px; }
.process-output h3 { margin: 0 0 5px; font-size: 13px; color: var(--text); }
.errors pre { overflow: auto; margin: 0; padding: 10px; border: 1px solid color-mix(in srgb, var(--red) 25%, var(--line)); border-radius: 8px; color: var(--text); background: var(--surface); white-space: pre-wrap; }
@media (max-width: 720px) { .check-details { grid-template-columns: 1fr; gap: 4px; } .check-details dd + dt { margin-top: 6px; } }
</style>
