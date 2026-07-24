/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useMemo } from 'react';
import { Tag, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import RESOURCE_CENTER_OPENAPI_SPEC from '../../../../../docs/openapi/resource-center.json';

const { Text } = Typography;
const HTTP_METHODS = new Set(['get', 'post', 'put', 'patch', 'delete']);

const OPERATIONS = Object.entries(RESOURCE_CENTER_OPENAPI_SPEC.paths).flatMap(
  ([path, pathItem]) =>
    Object.entries(pathItem)
      .filter(([method]) => HTTP_METHODS.has(method))
      .map(([method, operation]) => ({ method, path, operation })),
);

function resolveReference(reference) {
  if (!reference?.startsWith('#/')) return null;
  return reference
    .slice(2)
    .split('/')
    .reduce((value, segment) => value?.[segment], RESOURCE_CENTER_OPENAPI_SPEC);
}

function resolveSchema(schema) {
  if (!schema?.$ref) return schema || {};
  const resolved = resolveReference(schema.$ref) || {};
  return { ...resolved, ...schema, $ref: undefined };
}

function localizedDescription(value, language) {
  if (language?.startsWith('zh')) {
    return value?.['x-description-zh-CN'] || '';
  }
  return value?.description || '';
}

function schemaType(schema) {
  const resolved = resolveSchema(schema);
  if (resolved.oneOf) {
    return [...new Set(resolved.oneOf.map(schemaType))].join(' | ');
  }
  if (Array.isArray(resolved.type)) return resolved.type.join(' | ');
  if (resolved.type === 'array') return `array<${schemaType(resolved.items)}>`;
  if (resolved.const !== undefined) return typeof resolved.const;
  if (resolved.type) {
    return resolved.format
      ? `${resolved.type}<${resolved.format}>`
      : resolved.type;
  }
  if (resolved.properties || resolved.additionalProperties) return 'object';
  return 'any';
}

function schemaConstraints(schema) {
  const resolved = resolveSchema(schema);
  const constraints = [];
  if (resolved.const !== undefined) {
    constraints.push(`const: ${JSON.stringify(resolved.const)}`);
  }
  if (resolved.enum) constraints.push(`enum: ${resolved.enum.join(' | ')}`);
  if (resolved.default !== undefined) {
    constraints.push(`default: ${JSON.stringify(resolved.default)}`);
  }
  if (resolved.minimum !== undefined)
    constraints.push(`min: ${resolved.minimum}`);
  if (resolved.maximum !== undefined)
    constraints.push(`max: ${resolved.maximum}`);
  if (resolved.minLength !== undefined) {
    constraints.push(`minLength: ${resolved.minLength}`);
  }
  if (resolved.maxLength !== undefined) {
    constraints.push(`maxLength: ${resolved.maxLength}`);
  }
  if (resolved.minItems !== undefined) {
    constraints.push(`minItems: ${resolved.minItems}`);
  }
  if (resolved.maxItems !== undefined) {
    constraints.push(`maxItems: ${resolved.maxItems}`);
  }
  if (resolved.pattern) constraints.push(`pattern: ${resolved.pattern}`);
  return constraints;
}

function mergeRequirement(current, next) {
  if (!current) return next;
  if (current === next) return current;
  if (current === 'required' && next === 'required') return 'required';
  if (current === 'optional' && next === 'optional') return 'optional';
  return 'conditional';
}

function addRow(rows, row) {
  const existing = rows.find(
    (item) => item.path === row.path && item.location === row.location,
  );
  if (!existing) {
    rows.push(row);
    return;
  }
  existing.requirement = mergeRequirement(
    existing.requirement,
    row.requirement,
  );
  existing.type = [
    ...new Set(`${existing.type} | ${row.type}`.split(' | ')),
  ].join(' | ');
  if (!existing.description && row.description) {
    existing.description = row.description;
  }
  existing.constraints = [
    ...new Set([...existing.constraints, ...row.constraints]),
  ];
}

function childRequirement(parentRequirement, locallyRequired, variant) {
  if (variant) return locallyRequired ? 'conditional' : 'optional';
  if (!locallyRequired) return 'optional';
  return parentRequirement === 'required' ? 'required' : 'conditional';
}

function appendSchemaRows(
  rows,
  schema,
  path,
  requirement,
  location,
  language,
  options = {},
) {
  const resolved = resolveSchema(schema);
  const {
    includeCurrent = true,
    variant = false,
    commonVariantRequired = new Set(),
  } = options;

  if (includeCurrent) {
    addRow(rows, {
      path,
      location,
      type: schemaType(resolved),
      requirement,
      description: localizedDescription(resolved, language),
      constraints: schemaConstraints(resolved),
    });
  }

  if (resolved.oneOf) {
    const variants = resolved.oneOf.map(resolveSchema);
    const objectVariants = variants.filter(
      (candidate) => candidate.type !== 'null' && candidate.properties,
    );
    let sharedRequired = new Set();
    if (
      objectVariants.length === variants.length &&
      objectVariants.length > 0
    ) {
      sharedRequired = new Set(objectVariants[0].required || []);
      objectVariants.slice(1).forEach((candidate) => {
        const candidateRequired = new Set(candidate.required || []);
        sharedRequired = new Set(
          [...sharedRequired].filter((name) => candidateRequired.has(name)),
        );
      });
    }
    variants.forEach((variantSchema) => {
      if (variantSchema.type === 'null') return;
      appendSchemaRows(
        rows,
        variantSchema,
        path,
        requirement,
        location,
        language,
        {
          includeCurrent: false,
          variant: true,
          commonVariantRequired: sharedRequired,
        },
      );
    });
    return;
  }

  if (resolved.type === 'array' && resolved.items) {
    appendSchemaRows(
      rows,
      resolved.items,
      `${path}[]`,
      requirement,
      location,
      language,
      {
        includeCurrent: false,
        variant,
      },
    );
    return;
  }

  if (!resolved.properties) return;
  const required = new Set(resolved.required || []);
  Object.entries(resolved.properties).forEach(([name, property]) => {
    appendSchemaRows(
      rows,
      property,
      path ? `${path}.${name}` : name,
      childRequirement(
        requirement,
        required.has(name),
        variant && !commonVariantRequired.has(name),
      ),
      location,
      language,
      { variant },
    );
  });
}

function schemaRows(schema, language, location = 'body') {
  const rows = [];
  const resolved = resolveSchema(schema);
  appendSchemaRows(rows, schema, '', 'required', location, language, {
    includeCurrent: !resolved.properties && !resolved.oneOf,
  });
  return rows;
}

function parameterRows(parameters, language) {
  return (parameters || []).map((parameter) => ({
    path: parameter.name,
    location: parameter.in,
    type: schemaType(parameter.schema),
    requirement: parameter.required ? 'required' : 'optional',
    description: localizedDescription(parameter, language),
    constraints: schemaConstraints(parameter.schema),
  }));
}

function responseGroups(operation, language) {
  return Object.entries(operation.responses || {})
    .filter(([status]) => /^2/.test(status))
    .flatMap(([status, responseOrReference]) => {
      const response = responseOrReference.$ref
        ? resolveReference(responseOrReference.$ref) || {}
        : responseOrReference;
      const headerRows = Object.entries(response.headers || {}).map(
        ([name, header]) => ({
          path: name,
          location: 'response_header',
          type: schemaType(header.schema),
          requirement: header.required ? 'required' : 'optional',
          description: localizedDescription(header, language),
          constraints: schemaConstraints(header.schema),
        }),
      );
      const contentGroups = Object.entries(response.content || {}).map(
        ([contentType, media]) => ({
          key: `${status}-${contentType}`,
          title: `${status} · ${contentType}`,
          description: localizedDescription(response, language),
          rows: schemaRows(media.schema, language, 'response_body'),
        }),
      );
      if (contentGroups.length === 0) {
        contentGroups.push({
          key: `${status}-empty`,
          title: status,
          description: localizedDescription(response, language),
          rows: [],
        });
      }
      if (headerRows.length > 0) {
        contentGroups[0].rows = [...headerRows, ...contentGroups[0].rows];
      }
      return contentGroups;
    });
}

function RequirementTag({ value, t }) {
  const config = {
    required: { color: 'red', label: t('必填') },
    conditional: { color: 'orange', label: t('条件必填') },
    optional: { color: 'grey', label: t('可选') },
  }[value];
  return <Tag color={config.color}>{config.label}</Tag>;
}

function localizedConstraint(constraint, t) {
  const labels = {
    const: t('固定值'),
    enum: t('可选值'),
    default: t('默认值'),
    min: t('最小值'),
    max: t('最大值'),
    minLength: t('最短长度'),
    maxLength: t('最长长度'),
    minItems: t('最少数量'),
    maxItems: t('最多数量'),
    pattern: t('格式'),
  };
  const separator = constraint.indexOf(':');
  if (separator < 0) return constraint;
  const key = constraint.slice(0, separator);
  return `${labels[key] || key}:${constraint.slice(separator + 1)}`;
}

function localizedLocation(location, t) {
  const labels = {
    path: t('路径参数'),
    query: t('查询参数'),
    header: t('请求头'),
    body: t('请求体'),
    response_header: t('响应头'),
    response_body: t('响应体'),
  };
  return labels[location] || location;
}

function FieldNotes({ row, t }) {
  return (
    <div className='flex min-w-0 flex-col gap-1'>
      <div>
        <Tag color='blue' size='small'>
          {localizedLocation(row.location, t)}
        </Tag>
      </div>
      {row.constraints.length > 0 && (
        <Text type='tertiary' className='break-words font-mono text-xs'>
          {row.constraints
            .map((constraint) => localizedConstraint(constraint, t))
            .join(' · ')}
        </Text>
      )}
    </div>
  );
}

function FieldRows({ rows, t }) {
  if (rows.length === 0) {
    return <Text type='tertiary'>{t('无字段')}</Text>;
  }

  return (
    <>
      <div className='hidden max-w-full overflow-x-auto border-y border-solid border-semi-color-border md:block'>
        <table className='w-full min-w-[760px] border-collapse text-sm'>
          <thead>
            <tr className='bg-semi-color-fill-0'>
              {[t('名称'), t('类型'), t('是否必须'), t('描述'), t('备注')].map(
                (column) => (
                  <th
                    key={column}
                    className='border-0 border-b border-solid border-semi-color-border px-3 py-2 text-left font-medium text-semi-color-text-1'
                  >
                    {column}
                  </th>
                ),
              )}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={`${row.location}-${row.path}`}
                className='border-0 border-b border-solid border-semi-color-border last:border-b-0'
              >
                <td className='max-w-64 px-3 py-2 align-top'>
                  <Text code className='break-all'>
                    {row.path || t('响应体')}
                  </Text>
                </td>
                <td className='px-3 py-2 align-top'>
                  <Text code>{row.type}</Text>
                </td>
                <td className='px-3 py-2 align-top'>
                  <RequirementTag value={row.requirement} t={t} />
                </td>
                <td className='min-w-72 px-3 py-2 align-top'>
                  {row.description ? (
                    <Text className='break-words text-sm'>
                      {row.description}
                    </Text>
                  ) : (
                    <Text type='tertiary'>-</Text>
                  )}
                </td>
                <td className='min-w-56 px-3 py-2 align-top'>
                  <FieldNotes row={row} t={t} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className='border-y border-solid border-semi-color-border md:hidden'>
        {rows.map((row) => (
          <div
            key={`${row.location}-${row.path}`}
            className='flex min-w-0 flex-col gap-2 border-0 border-b border-solid border-semi-color-border px-1 py-3 last:border-b-0'
          >
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <Text code className='min-w-0 break-all'>
                {row.path || t('响应体')}
              </Text>
              <RequirementTag value={row.requirement} t={t} />
            </div>
            <div className='grid min-w-0 grid-cols-[72px_minmax(0,1fr)] gap-x-3 gap-y-2'>
              <Text type='tertiary'>{t('类型')}</Text>
              <Text code className='break-all'>
                {row.type}
              </Text>
              <Text type='tertiary'>{t('描述')}</Text>
              {row.description ? (
                <Text className='break-words'>{row.description}</Text>
              ) : (
                <Text type='tertiary'>-</Text>
              )}
              <Text type='tertiary'>{t('备注')}</Text>
              <FieldNotes row={row} t={t} />
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

function DefinitionGroup({ title, description, rows, t }) {
  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <div className='flex min-w-0 flex-col gap-1'>
        <Text strong>{title}</Text>
        {description && <Text type='tertiary'>{description}</Text>}
      </div>
      <FieldRows rows={rows} t={t} />
    </div>
  );
}

export function OperationSchemaDefinition({ operationId, title }) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage || i18n.language;
  const operationEntry = useMemo(
    () => OPERATIONS.find((item) => item.operation.operationId === operationId),
    [operationId],
  );
  if (!operationEntry) return null;

  const { operation } = operationEntry;
  const parameters = parameterRows(operation.parameters, language);
  const requestGroups = Object.entries(
    operation.requestBody?.content || {},
  ).map(([contentType, media]) => ({
    key: contentType,
    title: contentType,
    description: localizedDescription(resolveSchema(media.schema), language),
    rows: schemaRows(media.schema, language),
  }));
  const responses = responseGroups(operation, language);

  return (
    <div className='flex min-w-0 flex-col gap-5 border-0 border-t border-solid border-semi-color-border pt-4'>
      <Text strong>{title || t('请求与返回参数')}</Text>
      <div className='flex min-w-0 flex-col gap-3'>
        <Text strong>{t('请求参数')}</Text>
        {parameters.length > 0 && (
          <DefinitionGroup
            title={t('路径、查询与 Header')}
            rows={parameters}
            t={t}
          />
        )}
        {requestGroups.map((group) => (
          <DefinitionGroup key={group.key} {...group} t={t} />
        ))}
        {parameters.length === 0 && requestGroups.length === 0 && (
          <Text type='tertiary'>{t('无请求参数')}</Text>
        )}
      </div>

      <div className='flex min-w-0 flex-col gap-3'>
        <Text strong>{t('成功响应')}</Text>
        {responses.map((group) => (
          <DefinitionGroup key={group.key} {...group} t={t} />
        ))}
      </div>
    </div>
  );
}

export function StandaloneSchemaDefinition({ schemaName, title }) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage || i18n.language;
  const schema = RESOURCE_CENTER_OPENAPI_SPEC.components.schemas[schemaName];
  if (!schema) return null;
  return (
    <div className='flex min-w-0 flex-col gap-3 border-0 border-t border-solid border-semi-color-border pt-4'>
      <Text strong>{title || t('字段定义')}</Text>
      <DefinitionGroup
        title={schemaName}
        description={localizedDescription(schema, language)}
        rows={schemaRows(schema, language)}
        t={t}
      />
    </div>
  );
}
