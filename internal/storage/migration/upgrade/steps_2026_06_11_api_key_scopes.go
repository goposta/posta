/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package upgrade

import (
	"fmt"

	"gorm.io/gorm"
)

func applyAPIKeyScopes(tx *gorm.DB) error {
	if err := tx.Exec(
		`UPDATE api_keys SET scopes = '{send}' WHERE scopes IS NULL OR scopes = '{}'`,
	).Error; err != nil {
		return fmt.Errorf("backfill api_key scopes: %w", err)
	}
	return nil
}
