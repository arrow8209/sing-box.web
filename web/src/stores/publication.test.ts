import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import { usePublicationStore } from "./publication";
import { useDraftStore } from "./draft";
import { useProfilesStore } from "./profiles";
import { useCapabilitiesStore } from "./capabilities";
import type { Profile, ValidationResult } from "../api/types";

const profile: Profile = {
  id: "a".repeat(32),
  name: "家庭网关",
  slug: "home-gateway",
  draftRevision: 3,
  publishedRevision: null,
  publishedDigest: "",
  publishedAt: null,
  lastValidation: null,
  directoryHint: "/etc/sing-box/home-gateway",
  checkCommand: "sing-box check -C /etc/sing-box/home-gateway",
  draftReadOnly: false,
  draftError: "",
};

const validationResult: ValidationResult = {
  valid: true,
  issues: [],
  targetVersion: "1.12.0",
  installedVersion: "1.12.0",
  checkedAt: "2026-08-18T10:00:00Z",
};

describe("usePublicationStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    apiClient.clearCSRFToken();
    // 默认启用发布能力；能力禁用场景在专属用例中覆盖。
    const capabilities = useCapabilitiesStore();
    capabilities.snapshot = {
      capabilities: {
        "config.manage": true,
        "config.publish": true,
        "subscription.sync": false,
        "ruleset.sync": false,
        "core.lifecycle": false,
        "core.metrics": false,
      },
      singBox: {
        available: true,
        version: "1.12.0",
        targetVersion: "1.12.0",
        warning: "",
      },
    };
  });

  it("validate calls /validate and maps field issues without changing draft status", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    draft.draft = {
      profileId: profile.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "",
    };
    draft.etag = '"draft-3"';
    draft.status = "saved";

    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: validationResult,
      etag: null,
    });
    const publication = usePublicationStore();

    const result = await publication.validate();

    expect(request).toHaveBeenCalledWith(`/profiles/${profile.id}/validate`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    expect(result.valid).toBe(true);
    expect(draft.status).toBe("saved");
    expect(publication.lastValidation).toEqual(validationResult);
  });

  it("publish flushes autosave, requires ETag, never retries and updates the profile", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    draft.draft = {
      profileId: profile.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "",
    };
    draft.etag = '"draft-3"';
    draft.status = "saved";

    const receipt = {
      profileId: profile.id,
      revision: 3,
      digest: "digest-1",
      directory: "/etc/sing-box/home-gateway",
      publishedAt: "2026-08-18T10:00:00Z",
      validation: validationResult,
    };
    const save = vi.spyOn(draft, "save").mockResolvedValue(undefined);
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ data: receipt, etag: null });
    const publication = usePublicationStore();

    await publication.publish();

    expect(save).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledTimes(1);
    const [path, init] = request.mock.calls[0];
    expect(path).toBe(`/profiles/${profile.id}/publish`);
    expect(init).toEqual(
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ "If-Match": '"draft-3"' }),
      }),
    );
    expect(profiles.items[0].publishedRevision).toBe(3);
    expect(profiles.items[0].publishedDigest).toBe("digest-1");
  });

  it("publish without an ETag fails without issuing a request", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    draft.etag = null;
    draft.status = "saved";

    const request = vi.spyOn(apiClient, "request");
    const publication = usePublicationStore();

    await expect(publication.publish()).rejects.toThrow();
    expect(request).not.toHaveBeenCalled();
  });

  it("a failed check preserves the draft and surfaces the sanitized server message", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    const draftContent = { profileId: profile.id, revision: 3, content: { config: { log: { level: "info" } }, editorBuffers: {} }, savedAt: "" };
    draft.draft = { ...draftContent };
    draft.etag = '"draft-3"';
    draft.status = "saved";

    vi.spyOn(draft, "save").mockResolvedValue(undefined);
    vi.spyOn(apiClient, "request").mockRejectedValue(
      new APIClientError(422, {
        code: "singbox_check_failed",
        message: "sing-box check 失败",
        stderr: undefined as never,
      } as never),
    );
    const publication = usePublicationStore();

    await expect(publication.publish()).rejects.toBeInstanceOf(APIClientError);
    expect(draft.draft).toEqual(draftContent);
    expect(publication.lastError?.message).toBe("sing-box check 失败");
  });

  it("capability config.publish=false disables publish and shows the singBox warning", async () => {
    const capabilities = useCapabilitiesStore();
    capabilities.snapshot = {
      capabilities: {
        "config.manage": true,
        "config.publish": false,
        "subscription.sync": false,
        "ruleset.sync": false,
        "core.lifecycle": false,
        "core.metrics": false,
      },
      singBox: {
        available: false,
        version: "",
        targetVersion: "1.12.0",
        warning: "sing-box 不可用，配置发布已禁用",
      },
    };
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const publication = usePublicationStore();

    expect(publication.canPublish).toBe(false);
    expect(publication.publishBlockedReason).toContain("sing-box 不可用");
  });

  it("full export requires an explicit confirmation returning true", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;

    const confirm = vi.fn().mockResolvedValue(false);
    const request = vi.spyOn(apiClient, "request");
    const publication = usePublicationStore();

    await publication.downloadFull(confirm);
    expect(confirm).toHaveBeenCalled();
    expect(request).not.toHaveBeenCalled();

    confirm.mockResolvedValue(true);
    vi.spyOn(document, "createElement").mockReturnValue({
      setAttribute: vi.fn(),
      click: vi.fn(),
      download: "",
      href: "",
    } as unknown as HTMLElement);
	vi.spyOn(document.body, "appendChild").mockReturnValue(document.body);
	vi.spyOn(document.body, "removeChild").mockReturnValue(document.body);
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: vi.fn(() => "blob:mock"),
      revokeObjectURL: vi.fn(),
    });
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"log":{}}', { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await publication.downloadFull(confirm);

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/profiles/${profile.id}/export?mode=full&source=published&download=1`,
      expect.anything(),
    );
  });

  it("redacted export never invokes full mode", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;

    const fetchMock = vi.fn().mockResolvedValue(
      new Response('{"log":{"level":"info"},"experimental":{"clash_api":{"secret":"***"}}}', {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });

    const publication = usePublicationStore();
    await publication.copyRedacted();

    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain("mode=redacted");
    expect(url).not.toContain("mode=full");
    expect(writeText).toHaveBeenCalled();
  });
});
