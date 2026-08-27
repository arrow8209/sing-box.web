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
  <section v-if="validation?.errors?.length || error || (validation && !validation.valid)" class="errors" role="alert">
    <h2>配置检查结果</h2>
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

    <dl v-if="error?.details" class="check-details">
      <template v-if="error.details.command?.length">
        <dt>实际检查命令</dt>
        <dd><code>{{ formatCommand(error.details.command) }}</code></dd>
      </template>
      <template v-if="error.details.checkedConfigDirectory">
        <dt>实际送检目录</dt>
        <dd><code>{{ error.details.checkedConfigDirectory }}</code><small>相对服务工作目录；检查结束后已移至审核副本。</small></dd>
      </template>
      <template v-if="error.details.reviewConfigDirectory">
        <dt>后台审核目录</dt>
        <dd><code>{{ error.details.reviewConfigDirectory }}</code><small>相对服务工作目录。</small></dd>
      </template>
      <template v-if="error.details.reviewCommand?.length">
        <dt>后台复查命令</dt>
        <dd><code>{{ formatCommand(error.details.reviewCommand) }}</code></dd>
      </template>
      <template v-if="error.details.configFiles?.length">
        <dt>送检配置文件</dt>
        <dd><code>{{ error.details.configFiles.join(", ") }}</code></dd>
      </template>
      <template v-if="error.details.profileId || error.details.revision !== undefined">
        <dt>生成来源</dt>
        <dd><code v-if="error.details.profileId">档案 ID {{ error.details.profileId }}</code><span v-if="error.details.revision !== undefined">revision {{ error.details.revision }}</span><code v-if="error.details.configDigest">配置摘要 {{ error.details.configDigest }}</code></dd>
      </template>
      <template v-if="error.details.exitCode !== undefined">
        <dt>进程结果</dt>
        <dd>退出码 {{ error.details.exitCode }}<span v-if="error.details.timedOut">，执行超时</span></dd>
      </template>
    </dl>

    <p v-if="error?.details?.reviewError" class="detail-warning">未能保留审核副本：{{ error.details.reviewError }}</p>
    <p v-if="validation?.outputTruncated" class="detail-warning">核心输出超过 64 KiB，以下内容已截断。</p>
    <p v-if="error?.details?.outputTruncated" class="detail-warning">核心输出超过 64 KiB，以下内容已截断。</p>
    <div v-if="validation?.stdout" class="process-output"><h3>标准输出（stdout）</h3><pre>{{ validation.stdout }}</pre></div>
    <div v-if="validation?.stderr" class="process-output"><h3>标准错误（stderr）</h3><pre>{{ validation.stderr }}</pre></div>
    <div v-if="error?.details?.stdout" class="process-output"><h3>标准输出（stdout）</h3><pre>{{ error.details.stdout }}</pre></div>
    <div v-if="error?.details?.stderr" class="process-output"><h3>标准错误（stderr）</h3><pre>{{ error.details.stderr }}</pre></div>
    <div v-if="error?.details?.processError" class="process-output"><h3>进程错误</h3><pre>{{ error.details.processError }}</pre></div>
    <pre v-else-if="error?.output && !error.details?.stdout && !error.details?.stderr">{{ error.output }}</pre>
  </section>
</template>

<style scoped>
.errors { margin-top: 16px; padding: 14px; border: 1px solid color-mix(in srgb, var(--red) 40%, var(--line)); border-radius: var(--radius); color: var(--red); background: var(--red-soft); }
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
