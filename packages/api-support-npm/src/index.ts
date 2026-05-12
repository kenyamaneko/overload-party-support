// data/openapi.yaml から openapi-typescript で生成された components / paths の re-export。
// 利用者は本ファイルが re-export する schema 型を import すること。

import type { components, paths } from "./openapi.gen";

export type { components, paths };

type Schemas = components["schemas"];

export type HealthResponse = Schemas["HealthResponse"];
export type AnnouncementListResponse = Schemas["AnnouncementListResponse"];
export type AnnouncementSummary = Schemas["AnnouncementSummary"];
export type AnnouncementDetail = Schemas["AnnouncementDetail"];
export type AnnouncementType = Schemas["AnnouncementType"];
