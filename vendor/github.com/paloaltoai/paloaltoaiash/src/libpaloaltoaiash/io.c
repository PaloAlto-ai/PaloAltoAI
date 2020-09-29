/*
  This file is part of paaash.

  paaash is free software: you can redistribute it and/or modify
  it under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  paaash is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with paaash.  If not, see <http://www.gnu.org/licenses/>.
*/
/** @file io.c
 * @author Lefteris Karapetsas <lefteris@paadev.com>
 * @date 2015
 */
#include "io.h"
#include <string.h>
#include <stdio.h>
#include <errno.h>

enum paaash_io_rc paaash_io_prepare(
	char const* dirname,
	paaash_h256_t const seedhash,
	FILE** output_file,
	uint64_t file_size,
	bool force_create
)
{
	char mutable_name[DAG_MUTABLE_NAME_MAX_SIZE];
	enum paaash_io_rc ret = PAAASH_IO_FAIL;
	// reset errno before io calls
	errno = 0;

	// assert directory exists
	if (!paaash_mkdir(dirname)) {
		PAAASH_CRITICAL("Could not create the paaash directory");
		goto end;
	}

	paaash_io_mutable_name(PAAASH_REVISION, &seedhash, mutable_name);
	char* tmpfile = paaash_io_create_filename(dirname, mutable_name, strlen(mutable_name));
	if (!tmpfile) {
		PAAASH_CRITICAL("Could not create the full DAG pathname");
		goto end;
	}

	FILE *f;
	if (!force_create) {
		// try to open the file
		f = paaash_fopen(tmpfile, "rb+");
		if (f) {
			size_t found_size;
			if (!paaash_file_size(f, &found_size)) {
				fclose(f);
				PAAASH_CRITICAL("Could not query size of DAG file: \"%s\"", tmpfile);
				goto free_memo;
			}
			if (file_size != found_size - PAAASH_DAG_MAGIC_NUM_SIZE) {
				fclose(f);
				ret = PAAASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			// compare the magic number, no need to care about endianess since it's local
			uint64_t magic_num;
			if (fread(&magic_num, PAAASH_DAG_MAGIC_NUM_SIZE, 1, f) != 1) {
				// I/O error
				fclose(f);
				PAAASH_CRITICAL("Could not read from DAG file: \"%s\"", tmpfile);
				ret = PAAASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			if (magic_num != PAAASH_DAG_MAGIC_NUM) {
				fclose(f);
				ret = PAAASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			ret = PAAASH_IO_MEMO_MATCH;
			goto set_file;
		}
	}
	
	// file does not exist, will need to be created
	f = paaash_fopen(tmpfile, "wb+");
	if (!f) {
		PAAASH_CRITICAL("Could not create DAG file: \"%s\"", tmpfile);
		goto free_memo;
	}
	// make sure it's of the proper size
	if (fseek(f, (long int)(file_size + PAAASH_DAG_MAGIC_NUM_SIZE - 1), SEEK_SET) != 0) {
		fclose(f);
		PAAASH_CRITICAL("Could not seek to the end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	if (fputc('\n', f) == EOF) {
		fclose(f);
		PAAASH_CRITICAL("Could not write in the end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	if (fflush(f) != 0) {
		fclose(f);
		PAAASH_CRITICAL("Could not flush at end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	ret = PAAASH_IO_MEMO_MISMATCH;
	goto set_file;

	ret = PAAASH_IO_MEMO_MATCH;
set_file:
	*output_file = f;
free_memo:
	free(tmpfile);
end:
	return ret;
}
